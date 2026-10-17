package payments

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// Ledger entry types. Seller balances sum only BalanceTypes.
const (
	EntryCheckoutCreated  = "checkout.created"
	EntryPaymentSucceeded = "payment.succeeded"
	EntryPixReceived      = "pix.received"
	EntrySellerCredit     = "seller.credit"
	EntrySellerDebit      = "seller.debit"
	EntryTransferCreated  = "transfer.created"
	EntryTransferFailed   = "transfer.failed"
	EntryTransferReversed = "transfer.reversed"
	EntryRefund           = "refund"
	EntryRefundDue        = "refund.due"
	EntryOversellRefund   = "oversell.refund"
	EntryPayoutPaid       = "payout.paid"
)

// BalanceTypes are the entries that move a seller's in-app balance (money
// the platform holds for the seller: PIX sales, minus PIX refunds and paid
// payouts). Card sales go straight to the seller's Stripe account through
// transfers, so they never count here.
var BalanceTypes = []string{EntrySellerCredit, EntrySellerDebit, EntryPayoutPaid}

// Entry is one append-only ledger row. Amount is signed cents.
type Entry struct {
	Type          string
	OrderID       int64
	SellerOrderID int64
	ShopID        int64
	AmountCents   int64
	Provider      string
	ProviderRef   string
	// DedupeKey makes the write idempotent: a second entry with the same
	// key is silently skipped (racing webhook deliveries log once).
	DedupeKey string
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}

	return id
}

func nullString(s string) any {
	if s == "" {
		return nil
	}

	return s
}

// WriteEntry appends e inside the caller's transaction and reports whether
// it was written (false: an entry with the same dedupe key exists).
func WriteEntry(ctx context.Context, ex database.Executor, e Entry, now time.Time) (bool, error) {
	n, err := dbx.Exec(ctx, ex,
		`INSERT INTO ledger_entries (type, order_id, seller_order_id, shop_id, amount_cents, provider, provider_ref,
			dedupe_key, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (dedupe_key) DO NOTHING`,
		e.Type, nullID(e.OrderID), nullID(e.SellerOrderID), nullID(e.ShopID), e.AmountCents,
		nullString(e.Provider), nullString(e.ProviderRef), nullString(e.DedupeKey), now.UTC())
	if err != nil {
		return false, fmt.Errorf("ledger %s: %w", e.Type, err)
	}

	return n == 1, nil
}

// Balance is a shop's in-app balance: the sum of its BalanceTypes entries.
// It is never stored in a column.
func Balance(ctx context.Context, ex database.Executor, shopID int64) (int64, error) {
	var sum sql.NullInt64

	err := dbx.QueryRow(ctx, ex,
		`SELECT SUM(amount_cents) FROM ledger_entries WHERE shop_id = ? AND type IN (?, ?, ?)`,
		shopID, BalanceTypes[0], BalanceTypes[1], BalanceTypes[2]).Scan(&sum)
	if err != nil {
		return 0, fmt.Errorf("balance of shop %d: %w", shopID, err)
	}

	return sum.Int64, nil
}

// ProviderEvents records provider webhook events by id (unique per
// provider) so each one is processed once. Payloads are not stored here,
// only their SHA-256 digest.
type ProviderEvents struct {
	DB  *database.DB
	Now func() time.Time
}

// ErrEventProcessed is returned by Claim for an event already processed.
var ErrEventProcessed = errors.New("payments: event already processed")

// Claim records the event as received. It returns ErrEventProcessed when
// it was processed before; an event received but not processed (an
// earlier attempt failed) is returned for processing again, which is safe
// because every step is idempotent.
func (p ProviderEvents) Claim(ctx context.Context, provider, eventID, eventType string, payload []byte) error {
	sum := sha256.Sum256(payload)

	_, err := dbx.Exec(ctx, p.DB,
		`INSERT INTO provider_events (provider, event_id, type, payload_digest, received_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (provider, event_id) DO NOTHING`,
		provider, eventID, eventType, hex.EncodeToString(sum[:]), p.now())
	if err != nil {
		return fmt.Errorf("claim %s event: %w", provider, err)
	}

	var processed sql.NullTime

	err = dbx.QueryRow(ctx, p.DB, `SELECT processed_at FROM provider_events WHERE provider = ? AND event_id = ?`,
		provider, eventID).Scan(&processed)
	if err != nil {
		return fmt.Errorf("read %s event: %w", provider, err)
	}

	if processed.Valid {
		return ErrEventProcessed
	}

	return nil
}

// Done marks the event processed; only call it after processing succeeded.
func (p ProviderEvents) Done(ctx context.Context, provider, eventID string) error {
	_, err := dbx.Exec(ctx, p.DB,
		`UPDATE provider_events SET processed_at = ? WHERE provider = ? AND event_id = ? AND processed_at IS NULL`,
		p.now(), provider, eventID)
	if err != nil {
		return fmt.Errorf("mark %s event processed: %w", provider, err)
	}

	return nil
}

func (p ProviderEvents) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}

	return time.Now().UTC()
}
