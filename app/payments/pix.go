package payments

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/AlexGalhardo/vukem/auth/totp"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

// DefaultPixTTL is how long a PIX charge can be paid.
const DefaultPixTTL = 30 * time.Minute

func (s *Service) pixTTL() time.Duration {
	if s.PixTTL > 0 {
		return s.PixTTL
	}

	return DefaultPixTTL
}

func pixQRKey(orderID int64) string { return "pix-qr:" + strconv.FormatInt(orderID, 10) }

// StartPix creates the AbacatePay PIX charge of a pending PIX order and
// stores its id, copy-and-paste code and expiry. AbacatePay deduplicates
// on externalId (the order public id).
func (s *Service) StartPix(ctx context.Context, o Order) error {
	if s.Pix == nil {
		return ErrPixDisabled
	}

	if o.Method != MethodPix || o.Status != OrderPending || o.PixChargeID != "" {
		return fmt.Errorf("payments: order %d is not a new pending PIX order", o.ID)
	}

	ch, err := s.Pix.CreatePixCharge(ctx, PixChargeParams{
		AmountCents: o.TotalCents, ExternalID: o.PublicID, Description: "Order " + o.PublicID, ExpiresIn: s.pixTTL(),
	})
	if err != nil {
		return fmt.Errorf("create PIX charge for order %d: %w", o.ID, err)
	}

	if _, err := dbx.Exec(ctx, s.DB, `UPDATE orders SET abacatepay_charge_id = ?, pix_br_code = ?, pix_expires_at = ?,
		updated_at = ? WHERE id = ? AND abacatepay_charge_id IS NULL`,
		ch.ID, ch.BRCode, ch.ExpiresAt.UTC(), s.now(), o.ID); err != nil {
		return fmt.Errorf("store PIX charge: %w", err)
	}

	if s.Cache != nil && ch.QRImage != "" {
		_ = s.Cache.Set(ctx, pixQRKey(o.ID), []byte(ch.QRImage), s.pixTTL()+time.Hour) // can be redrawn from the code
	}

	return nil
}

// PixQR returns the QR image of an order's PIX code as a data: URI (never
// HTML): the provider's PNG when cached, else drawn from the code.
func (s *Service) PixQR(ctx context.Context, o Order) string {
	if s.Cache != nil {
		if b, ok, err := s.Cache.Get(ctx, pixQRKey(o.ID)); err == nil && ok {
			return string(b)
		}
	}

	if o.PixBRCode == "" {
		return ""
	}

	// workaround(vukem): totp.QR stops at QR version 10 (213 bytes).
	// Most BR Codes fit; longer ones show the copy-and-paste code only.
	svg, err := totp.QR(o.PixBRCode, 256)
	if err != nil {
		return ""
	}

	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// SimulatePix pays a PIX charge in AbacatePay's dev mode (never in
// production); the webhook then fulfils the order.
func (s *Service) SimulatePix(ctx context.Context, o Order) error {
	if s.Pix == nil {
		return ErrPixDisabled
	}

	if s.Config.Production || o.PixChargeID == "" {
		return errors.New("payments: simulate-payment is a dev-mode tool")
	}

	return s.Pix.SimulatePayment(ctx, o.PixChargeID)
}

// PixEvent is a verified AbacatePay webhook.
type PixEvent struct {
	ID      string          `json:"id"`
	Event   string          `json:"event"`
	DevMode bool            `json:"devMode"`
	Data    json.RawMessage `json:"data"`
}

// pixCharge is the charge inside an event; the v2 payload nests it under
// a key in some events and sends it bare in others, so both are read.
type pixCharge struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Amount     int64  `json:"amount"`
	PaidAmount int64  `json:"paidAmount"`
	ExternalID string `json:"externalId"`
}

func (e PixEvent) charge() pixCharge {
	var bare pixCharge
	_ = json.Unmarshal(e.Data, &bare)

	if bare.ID != "" {
		return bare
	}

	var nested map[string]json.RawMessage
	_ = json.Unmarshal(e.Data, &nested)

	for _, k := range []string{"transparent", "pixQrCode", "billing", "payment"} {
		var c pixCharge
		if raw, ok := nested[k]; ok && json.Unmarshal(raw, &c) == nil && c.ID != "" {
			return c
		}
	}

	return pixCharge{}
}

// HandlePixEvent processes one verified AbacatePay event idempotently.
func (s *Service) HandlePixEvent(ctx context.Context, ev PixEvent) error {
	if ev.Event != "transparent.completed" && ev.Event != "billing.paid" {
		return nil
	}

	if s.Config.Production && ev.DevMode {
		s.log().WarnContext(ctx, "dev-mode PIX event in production", "event", "payments.pix_devmode", "id", ev.ID)
		return nil
	}

	ch := ev.charge()
	if ch.ID == "" || (ch.Status != "" && ch.Status != "PAID") {
		return nil
	}

	o, err := s.orderWhere(ctx, s.DB, `abacatepay_charge_id = ?`, ch.ID)
	if errors.Is(err, orm.ErrNotFound) {
		s.log().WarnContext(ctx, "PIX event for unknown charge", "event", "payments.unknown_charge", "charge", ch.ID)
		return nil
	}

	if err != nil {
		return err
	}

	amount := ch.PaidAmount
	if amount == 0 {
		amount = ch.Amount
	}

	if (ch.ExternalID != "" && ch.ExternalID != o.PublicID) || (amount != 0 && amount != o.TotalCents) {
		s.log().ErrorContext(ctx, "PIX payment does not match its order", "event", "payments.amount_mismatch",
			"order", o.ID, "charge", ch.ID, "amount", amount, "want", o.TotalCents)

		return nil
	}

	return s.markPaid(ctx, o, payment{method: MethodPix, provider: ProviderAbacatePay, providerRef: ch.ID})
}

// ExpirePix expires unpaid PIX orders past their deadline (pending →
// expired; their seller orders are canceled). Stock was never reserved, so
// nothing is released. It returns how many orders expired.
func (s *Service) ExpirePix(ctx context.Context) (int64, error) {
	var n int64

	err := s.DB.Tx(ctx, func(tx *database.Tx) error {
		now := s.now()

		var err error

		n, err = dbx.Exec(ctx, tx, `UPDATE orders SET status = ?, updated_at = ?
			WHERE payment_method = ? AND status = ? AND pix_expires_at IS NOT NULL AND pix_expires_at < ?`,
			OrderExpired, now, MethodPix, OrderPending, now)
		if err != nil {
			return fmt.Errorf("expire PIX orders: %w", err)
		}

		_, err = dbx.Exec(ctx, tx, `UPDATE seller_orders SET status = ?, updated_at = ?
			WHERE status = ? AND order_id IN (SELECT id FROM orders WHERE payment_method = ? AND status = ?)`,
			SellerCanceled, now, SellerPending, MethodPix, OrderExpired)

		return err
	})

	return n, err
}
