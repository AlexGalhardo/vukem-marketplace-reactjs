package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// Payout statuses.
const (
	PayoutRequested = "requested"
	PayoutPaid      = "paid"
	PayoutRejected  = "rejected"
)

// MinPayoutCents is the smallest payout a seller can request (R$ 10).
const MinPayoutCents = 1000

// Payout errors shown on the forms.
var (
	ErrPayoutTooSmall    = errs.NewValidation().Add("amountCents", "checkout.payouts.errors.too_small")
	ErrPayoutOverBalance = errs.NewValidation().Add("amountCents", "checkout.payouts.errors.over_balance")
	ErrPayoutNotPending  = errs.NewValidation().Add("payout", "checkout.payouts.errors.not_pending")
	ErrEndToEndID        = errs.NewValidation().Add("endToEndId", "checkout.payouts.errors.e2e")
	ErrEndToEndIDUsed    = errs.NewValidation().Add("endToEndId", "checkout.payouts.errors.e2e_used")
)

// endToEndRe is the BCB end-to-end id: E, the payer's ISPB (8 digits), a
// timestamp (yyyyMMddHHmm) and 11 alphanumerics.
var endToEndRe = regexp.MustCompile(`^E\d{8}\d{12}[A-Za-z0-9]{11}$`)

// PayoutRequest is a seller's payout request.
type PayoutRequest struct {
	ID             int64     `json:"id"`
	ShopID         int64     `json:"shopId"`
	ShopName       string    `json:"shopName"`
	AmountCents    int64     `json:"amountCents"`
	Status         string    `json:"status"`
	EndToEndID     string    `json:"endToEndId"`
	RejectedReason string    `json:"rejectedReason"`
	CreatedAt      time.Time `json:"createdAt"`
	PaidAt         time.Time `json:"paidAt,omitzero"`
}

// BalanceSummary is a shop's money as the seller sees it.
type BalanceSummary struct {
	// BalanceCents is the ledger sum (PIX sales held by the platform).
	BalanceCents int64 `json:"balanceCents"`
	// RequestedCents is awaiting payment by an admin.
	RequestedCents int64 `json:"requestedCents"`
	// AvailableCents can be requested now.
	AvailableCents int64 `json:"availableCents"`
}

func requested(ctx context.Context, ex database.Executor, shopID int64) (int64, error) {
	var sum sql.NullInt64

	err := dbx.QueryRow(ctx, ex, `SELECT SUM(amount_cents) FROM payouts WHERE shop_id = ? AND status = ?`,
		shopID, PayoutRequested).Scan(&sum)

	return sum.Int64, err
}

// Summary returns a shop's balance, pending requests and available amount.
func (s *Service) Summary(ctx context.Context, shopID int64) (BalanceSummary, error) {
	bal, err := Balance(ctx, s.DB, shopID)
	if err != nil {
		return BalanceSummary{}, err
	}

	req, err := requested(ctx, s.DB, shopID)
	if err != nil {
		return BalanceSummary{}, fmt.Errorf("requested payouts: %w", err)
	}

	return BalanceSummary{BalanceCents: bal, RequestedCents: req, AvailableCents: max(bal-req, 0)}, nil
}

// RequestPayout asks for amountCents of the shop's available balance. The
// shop row is locked so two concurrent requests cannot both spend it.
func (s *Service) RequestPayout(ctx context.Context, shopID, amountCents int64) error {
	if amountCents < MinPayoutCents {
		return ErrPayoutTooSmall
	}

	return s.DB.Tx(ctx, func(tx *database.Tx) error {
		var id int64
		if err := dbx.QueryRow(ctx, tx, `SELECT id FROM shops WHERE id = ?`+dbx.ForUpdate(tx.Dialect()), shopID).
			Scan(&id); err != nil {
			return fmt.Errorf("lock shop %d: %w", shopID, err)
		}

		bal, err := Balance(ctx, tx, shopID)
		if err != nil {
			return err
		}

		req, err := requested(ctx, tx, shopID)
		if err != nil {
			return err
		}

		if amountCents > bal-req {
			return ErrPayoutOverBalance
		}

		now := s.now()
		_, err = dbx.Exec(ctx, tx, `INSERT INTO payouts (shop_id, amount_cents, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`, shopID, amountCents, PayoutRequested, now, now)

		return err
	})
}

const payoutSelect = `SELECT p.id, p.shop_id, s.name, p.amount_cents, p.status, p.pix_end_to_end_id,
	p.rejection_reason, p.created_at, p.paid_at FROM payouts p JOIN shops s ON s.id = p.shop_id`

func (s *Service) payouts(ctx context.Context, where string, args ...any) ([]PayoutRequest, error) {
	rows, err := s.DB.QueryContext(ctx, dbx.Rebind(s.DB.Dialect(), payoutSelect+` WHERE `+where+
		` ORDER BY p.id DESC LIMIT 100`), args...)
	if err != nil {
		return nil, fmt.Errorf("list payouts: %w", err)
	}
	defer rows.Close()

	out := []PayoutRequest{}

	for rows.Next() {
		var (
			p           PayoutRequest
			e2e, reason sql.NullString
			paid        sql.NullTime
		)

		if err := rows.Scan(&p.ID, &p.ShopID, &p.ShopName, &p.AmountCents, &p.Status, &e2e, &reason, &p.CreatedAt,
			&paid); err != nil {
			return nil, fmt.Errorf("scan payout: %w", err)
		}

		p.EndToEndID, p.RejectedReason, p.PaidAt = e2e.String, reason.String, paid.Time

		out = append(out, p)
	}

	return out, rows.Err()
}

// ShopPayouts lists a shop's latest payouts.
func (s *Service) ShopPayouts(ctx context.Context, shopID int64) ([]PayoutRequest, error) {
	return s.payouts(ctx, `p.shop_id = ?`, shopID)
}

// PayoutsByStatus lists the latest payouts with a status (admin).
func (s *Service) PayoutsByStatus(ctx context.Context, status string) ([]PayoutRequest, error) {
	return s.payouts(ctx, `p.status = ?`, status)
}

// MarkPayoutPaid records that an admin paid a requested payout by PIX
// outside the app: the end-to-end id (unique) and a negative payout.paid
// ledger row, in one transaction with the conditional status change.
func (s *Service) MarkPayoutPaid(ctx context.Context, adminID, payoutID int64, endToEndID string) error {
	e2e := strings.TrimSpace(endToEndID)
	if !endToEndRe.MatchString(e2e) {
		return ErrEndToEndID
	}

	err := s.DB.Tx(ctx, func(tx *database.Tx) error {
		var shopID, amount int64

		err := dbx.QueryRow(ctx, tx, `SELECT shop_id, amount_cents FROM payouts WHERE id = ? AND status = ?`+
			dbx.ForUpdate(tx.Dialect()), payoutID, PayoutRequested).Scan(&shopID, &amount)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPayoutNotPending
		}

		if err != nil {
			return err
		}

		now := s.now()

		n, err := dbx.Exec(ctx, tx, `UPDATE payouts SET status = ?, pix_end_to_end_id = ?, paid_by = ?, paid_at = ?,
			updated_at = ? WHERE id = ? AND status = ?`, PayoutPaid, e2e, adminID, now, now, payoutID, PayoutRequested)
		if err != nil {
			return err
		}

		if n == 0 {
			return ErrPayoutNotPending
		}

		_, err = WriteEntry(ctx, tx, Entry{
			Type: EntryPayoutPaid, ShopID: shopID, AmountCents: -amount, Provider: "pix", ProviderRef: e2e,
			DedupeKey: EntryPayoutPaid + ":" + strconv.FormatInt(payoutID, 10),
		}, now)

		return err
	})
	if dbx.IsUniqueViolation(err) {
		return ErrEndToEndIDUsed
	}

	return err
}

// RejectPayout declines a requested payout; the amount becomes available
// again (nothing was written to the ledger).
func (s *Service) RejectPayout(ctx context.Context, payoutID int64, reason string) error {
	n, err := dbx.Exec(ctx, s.DB, `UPDATE payouts SET status = ?, rejection_reason = ?, updated_at = ?
		WHERE id = ? AND status = ?`, PayoutRejected, strings.TrimSpace(reason), s.now(), payoutID, PayoutRequested)
	if err != nil {
		return fmt.Errorf("reject payout %d: %w", payoutID, err)
	}

	if n == 0 {
		return ErrPayoutNotPending
	}

	return nil
}

// PayoutExists reports whether a payout id exists (admin 404s).
func (s *Service) PayoutExists(ctx context.Context, payoutID int64) error {
	var id int64

	err := dbx.QueryRow(ctx, s.DB, `SELECT id FROM payouts WHERE id = ?`, payoutID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return orm.ErrNotFound
	}

	return err
}
