package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	vstripe "github.com/AlexGalhardo/vukem/contrib/stripe"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// ErrCardDisabled is returned when Stripe is not configured.
var ErrCardDisabled = errors.New("payments: card payments are not configured")

// ErrPixDisabled is returned when AbacatePay is not configured.
var ErrPixDisabled = errors.New("payments: PIX is not configured")

// StartCard opens (or reopens: the idempotency key returns the same
// session) a Stripe Checkout Session for a pending card order and returns
// its URL. Line items are built from the order's snapshots, never from the
// client.
func (s *Service) StartCard(ctx context.Context, o Order, email string) (string, error) {
	if s.Stripe == nil {
		return "", ErrCardDisabled
	}

	if o.Method != MethodCard || o.Status != OrderPending {
		return "", fmt.Errorf("payments: order %d is not a pending card order", o.ID)
	}

	items, err := s.orderItems(ctx, s.DB, o.ID)
	if err != nil {
		return "", err
	}

	lines := make([]LineItem, 0, len(items)+1)
	for _, it := range items {
		lines = append(lines, LineItem{Name: it.Title, UnitCents: it.PriceCents, Quantity: it.Quantity})
	}

	if o.ShippingCents > 0 {
		lines = append(lines, LineItem{Name: "Shipping / Frete", UnitCents: o.ShippingCents, Quantity: 1})
	}

	sess, err := s.Stripe.CreateCheckoutSession(ctx, CheckoutSessionParams{
		OrderPublicID: o.PublicID,
		CustomerEmail: email,
		LineItems:     lines,
		SuccessURL:    s.Config.AppURL + "/checkout/success?order=" + o.PublicID,
		CancelURL:     s.Config.AppURL + "/checkout/payment?canceled=" + o.PublicID,
		Currency:      Currency,
	}, "checkout-"+strconv.FormatInt(o.ID, 10))
	if err != nil {
		return "", fmt.Errorf("create checkout session for order %d: %w", o.ID, err)
	}

	if _, err := dbx.Exec(ctx, s.DB, `UPDATE orders SET stripe_checkout_session_id = ?, updated_at = ?
		WHERE id = ? AND stripe_checkout_session_id IS NULL`, sess.ID, s.now(), o.ID); err != nil {
		return "", fmt.Errorf("store checkout session: %w", err)
	}

	return sess.URL, nil
}

// checkoutSession is the part of the webhook's session object the app reads.
type checkoutSession struct {
	ID            string            `json:"id"`
	PaymentStatus string            `json:"payment_status"`
	PaymentIntent string            `json:"payment_intent"`
	AmountTotal   int64             `json:"amount_total"`
	Currency      string            `json:"currency"`
	Metadata      map[string]string `json:"metadata"`
}

// HandleStripeEvent processes one verified Stripe event. It is idempotent:
// a replayed or reordered event changes nothing that was already done,
// and an error makes Stripe redeliver (transfers left undone are retried).
func (s *Service) HandleStripeEvent(ctx context.Context, ev vstripe.Event) error {
	switch ev.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		cs, err := vstripe.Decode[checkoutSession](ev)
		if err != nil {
			return fmt.Errorf("decode session: %w", err)
		}

		return s.cardPaid(ctx, cs)
	case "checkout.session.expired", "checkout.session.async_payment_failed":
		cs, err := vstripe.Decode[checkoutSession](ev)
		if err != nil {
			return fmt.Errorf("decode session: %w", err)
		}

		return s.cancelCardOrder(ctx, cs.ID)
	case "account.updated":
		var acct struct {
			ID string `json:"id"`
		}

		if err := json.Unmarshal(ev.Data.Object, &acct); err != nil || acct.ID == "" {
			return fmt.Errorf("decode account: %w", err)
		}

		return s.SyncAccount(ctx, acct.ID)
	case "charge.dispute.created", "charge.refunded":
		// shortcut: disputes and Dashboard refunds are logged only; reconcile them in the admin phase (M.2).
		s.log().WarnContext(ctx, "stripe event needs review", "event", "payments.stripe_review", "type", ev.Type,
			"id", ev.ID)

		return nil
	default:
		return nil
	}
}

// cardPaid fulfils the order of a paid Checkout Session.
func (s *Service) cardPaid(ctx context.Context, cs checkoutSession) error {
	if cs.PaymentStatus != "paid" {
		return nil // delayed methods confirm with async_payment_succeeded
	}

	// The session must be the one this app created for the order: a
	// session id is bound to exactly one order at creation.
	o, err := s.orderWhere(ctx, s.DB, `stripe_checkout_session_id = ?`, cs.ID)
	if errors.Is(err, orm.ErrNotFound) {
		s.log().WarnContext(ctx, "stripe session for unknown order", "event", "payments.unknown_session",
			"session", cs.ID)

		return nil
	}

	if err != nil {
		return err
	}

	if cs.Metadata["order_id"] != o.PublicID || cs.AmountTotal != o.TotalCents || cs.Currency != Currency {
		s.log().ErrorContext(ctx, "stripe session does not match its order", "event", "payments.amount_mismatch",
			"order", o.ID, "session", cs.ID, "amount", cs.AmountTotal, "want", o.TotalCents)

		return nil // never fulfil a mismatching payment; acknowledged for manual review
	}

	if err := s.markPaid(
		ctx,
		o,
		payment{method: MethodCard, provider: ProviderStripe, providerRef: cs.PaymentIntent},
	); err != nil {
		return err
	}

	return s.settleCard(ctx, o.ID)
}

// cancelCardOrder cancels a card order whose session expired unpaid.
func (s *Service) cancelCardOrder(ctx context.Context, sessionID string) error {
	return s.DB.Tx(ctx, func(tx *database.Tx) error {
		o, err := s.orderWhere(ctx, tx, `stripe_checkout_session_id = ?`, sessionID)
		if errors.Is(err, orm.ErrNotFound) {
			return nil // unknown session: nothing to cancel
		}

		if err != nil {
			return err
		}

		n, err := dbx.Exec(ctx, tx, `UPDATE orders SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
			OrderCanceled, s.now(), o.ID, OrderPending)
		if err != nil || n == 0 {
			return err
		}

		_, err = dbx.Exec(
			ctx,
			tx,
			`UPDATE seller_orders SET status = ?, updated_at = ? WHERE order_id = ? AND status = ?`,
			SellerCanceled,
			s.now(),
			o.ID,
			SellerPending,
		)

		return err
	})
}

// settleCard finishes a paid card order from its current state, so a
// retry resumes where a failure stopped: refunds for oversold lines, then
// one transfer per seller order to its Express account.
func (s *Service) settleCard(ctx context.Context, orderID int64) error {
	o, err := s.OrderByID(ctx, orderID)
	if err != nil {
		return err
	}

	if o.Method != MethodCard || o.PaymentIntentID == "" ||
		(o.Status != OrderPaid && o.Status != OrderPartiallyRefunded) {
		return nil
	}

	if err := s.refundOversold(ctx, o); err != nil {
		return err
	}

	sellers, err := s.sellerOrders(ctx, s.DB, o.ID)
	if err != nil {
		return err
	}

	var (
		charge string
		errs   []error
	)

	for _, so := range sellers {
		if so.Status != SellerPaid || so.StripeTransferID != "" || so.PayoutCents <= 0 {
			continue
		}

		if charge == "" {
			pi, err := s.Stripe.GetPaymentIntent(ctx, o.PaymentIntentID)
			if err != nil {
				return fmt.Errorf("payment intent of order %d: %w", o.ID, err)
			}

			charge = pi.LatestCharge
		}

		if err := s.transfer(ctx, o, so, charge); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (s *Service) transfer(ctx context.Context, o Order, so SellerOrder, charge string) error {
	idem := "transfer-" + strconv.FormatInt(so.ID, 10)

	fail := func(cause error) error {
		_, _ = WriteEntry(ctx, s.DB, Entry{
			Type: EntryTransferFailed, OrderID: o.ID, SellerOrderID: so.ID, ShopID: so.ShopID,
			AmountCents: so.PayoutCents, Provider: ProviderStripe,
		}, s.now())

		return fmt.Errorf("transfer seller order %d: %w", so.ID, cause)
	}

	if so.ShopAccountID == "" {
		return fail(errors.New("shop has no Stripe account"))
	}

	tr, err := s.Stripe.CreateTransfer(ctx, TransferParams{
		AmountCents: so.PayoutCents, Currency: Currency, Destination: so.ShopAccountID,
		TransferGroup: o.PublicID, SourceTransaction: charge, SellerOrderID: so.ID,
	}, idem)
	if err != nil {
		return fail(err)
	}

	return s.DB.Tx(ctx, func(tx *database.Tx) error {
		if _, err := dbx.Exec(ctx, tx, `UPDATE seller_orders SET stripe_transfer_id = ?, updated_at = ?
			WHERE id = ? AND stripe_transfer_id IS NULL`, tr.ID, s.now(), so.ID); err != nil {
			return err
		}

		_, err := WriteEntry(ctx, tx, Entry{
			Type: EntryTransferCreated, OrderID: o.ID, SellerOrderID: so.ID, ShopID: so.ShopID,
			AmountCents: so.PayoutCents, Provider: ProviderStripe, ProviderRef: tr.ID,
			DedupeKey: EntryTransferCreated + ":" + tr.ID,
		}, s.now())

		return err
	})
}

// refundOversold refunds, once per seller order, the lines that could not
// be served because stock ran out between checkout and payment.
func (s *Service) refundOversold(ctx context.Context, o Order) error {
	list, err := s.oversold(ctx, o.ID)
	if err != nil {
		return err
	}

	for _, w := range list {
		key := "refund-oversell-" + strconv.FormatInt(w.soID, 10)

		// Stripe forgets idempotency keys after 24h: the ledger is the record.
		var done int
		if err := dbx.QueryRow(ctx, s.DB, `SELECT COUNT(*) FROM ledger_entries WHERE dedupe_key = ?`, key).
			Scan(&done); err != nil {
			return err
		}

		if done > 0 {
			continue
		}

		rf, err := s.Stripe.CreateRefund(ctx, RefundParams{
			PaymentIntent: o.PaymentIntentID, AmountCents: -w.cents, SellerOrderID: w.soID, Reason: "oversold",
		}, key)
		if err != nil {
			return fmt.Errorf("refund oversold seller order %d: %w", w.soID, err)
		}

		if _, err := WriteEntry(ctx, s.DB, Entry{
			Type: EntryRefund, OrderID: o.ID, SellerOrderID: w.soID, ShopID: w.shopID, AmountCents: w.cents,
			Provider: ProviderStripe, ProviderRef: rf.ID, DedupeKey: key,
		}, s.now()); err != nil {
			return err
		}
	}

	return nil
}

type owed struct{ soID, shopID, cents int64 }

// oversold lists the oversell.refund entries of an order.
func (s *Service) oversold(ctx context.Context, orderID int64) ([]owed, error) {
	rows, err := s.DB.QueryContext(ctx, database.Rebind(s.DB.Dialect(),
		`SELECT seller_order_id, shop_id, amount_cents FROM ledger_entries WHERE order_id = ? AND type = ?`),
		orderID, EntryOversellRefund)
	if err != nil {
		return nil, fmt.Errorf("oversold lines: %w", err)
	}
	defer rows.Close()

	var list []owed

	for rows.Next() {
		var w owed
		if err := rows.Scan(&w.soID, &w.shopID, &w.cents); err != nil {
			return nil, err
		}

		list = append(list, w)
	}

	return list, rows.Err()
}
