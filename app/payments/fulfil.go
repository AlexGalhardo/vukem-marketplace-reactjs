package payments

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

// payment describes a confirmed payment from a provider webhook.
type payment struct {
	method      string
	provider    string
	providerRef string // PaymentIntent id (card) or AbacatePay charge id (PIX)
}

// markPaid moves an order to paid in one transaction: the conditional
// status change (a second delivery changes nothing), the seller orders,
// the stock decrement with a stock >= qty guard (an oversold line is
// refunded instead), the buyer's cart and the ledger. When
// the order was paid before, it changes nothing.
func (s *Service) markPaid(ctx context.Context, o Order, p payment) error {
	from := []any{OrderPending}
	if p.method == MethodPix {
		// The money arrived even if the expiry job ran first.
		from = append(from, OrderExpired)
	}

	return s.DB.Tx(ctx, func(tx *database.Tx) error {
		now := s.now()

		args := []any{OrderPaid, now, now, nullString(p.providerRef), o.ID, p.method}
		args = append(args, from...)

		inList := "?"
		if len(from) == 2 {
			inList = "?, ?"
		}

		intentCol := "stripe_payment_intent_id"
		if p.method == MethodPix {
			intentCol = "abacatepay_charge_id"
		}

		n, err := dbx.Exec(ctx, tx, `UPDATE orders SET status = ?, paid_at = ?, updated_at = ?, `+intentCol+
			` = COALESCE(`+intentCol+`, ?) WHERE id = ? AND payment_method = ? AND status IN (`+inList+`)`, args...)
		if err != nil {
			return fmt.Errorf("mark order %d paid: %w", o.ID, err)
		}

		if n == 0 {
			return nil // already paid, refunded or canceled: nothing to do
		}

		if _, err := dbx.Exec(ctx, tx, `UPDATE seller_orders SET status = ?, updated_at = ?
			WHERE order_id = ? AND status IN (?, ?)`, SellerPaid, now, o.ID, SellerPending, SellerCanceled); err != nil {
			return fmt.Errorf("mark seller orders paid: %w", err)
		}

		return s.fulfil(ctx, tx, o, p)
	})
}

func (s *Service) fulfil(ctx context.Context, tx *database.Tx, o Order, p payment) error {
	now := s.now()

	items, err := s.orderItems(ctx, tx, o.ID)
	if err != nil {
		return err
	}

	oversold := map[int64]int64{} // seller order id → cents of lines that could not be served

	var productIDs []any

	for _, it := range items {
		if it.ProductID == 0 {
			oversold[it.SellerOrderID] += it.PriceCents * int64(it.Quantity)
			continue
		}

		productIDs = append(productIDs, it.ProductID)

		n, err := dbx.Exec(ctx, tx, `UPDATE products SET stock = stock - ?, updated_at = ? WHERE id = ? AND stock >= ?`,
			it.Quantity, now, it.ProductID, it.Quantity)
		if err != nil {
			return fmt.Errorf("decrement stock of %d: %w", it.ProductID, err)
		}

		if n == 0 {
			oversold[it.SellerOrderID] += it.PriceCents * int64(it.Quantity)
		}
	}

	if len(productIDs) > 0 {
		q := `DELETE FROM cart_items WHERE user_id = ? AND product_id IN (?` + strings.Repeat(
			", ?",
			len(productIDs)-1,
		) + `)`
		if _, err := dbx.Exec(ctx, tx, q, append([]any{o.BuyerID}, productIDs...)...); err != nil {
			return fmt.Errorf("clear cart: %w", err)
		}
	}

	entryType := EntryPaymentSucceeded
	if p.method == MethodPix {
		entryType = EntryPixReceived
	}

	if _, err := WriteEntry(ctx, tx, Entry{
		Type: entryType, OrderID: o.ID, AmountCents: o.TotalCents, Provider: p.provider, ProviderRef: p.providerRef,
		DedupeKey: entryType + ":" + strconv.FormatInt(o.ID, 10),
	}, now); err != nil {
		return err
	}

	sellers, err := s.sellerOrders(ctx, tx, o.ID)
	if err != nil {
		return err
	}

	for _, so := range sellers {
		if cents := oversold[so.ID]; cents > 0 {
			if so, err = s.shrinkSellerOrder(ctx, tx, so, cents); err != nil {
				return err
			}
		}

		if p.method == MethodPix && so.PayoutCents > 0 {
			if _, err := WriteEntry(ctx, tx, Entry{
				Type: EntrySellerCredit, OrderID: o.ID, SellerOrderID: so.ID, ShopID: so.ShopID,
				AmountCents: so.PayoutCents, Provider: p.provider, ProviderRef: p.providerRef,
				DedupeKey: EntrySellerCredit + ":" + strconv.FormatInt(so.ID, 10),
			}, now); err != nil {
				return err
			}
		}
	}

	return nil
}

// shrinkSellerOrder removes oversold lines from a seller order's money and
// logs the amount owed back to the buyer (card: refunded automatically by
// settleCard; PIX: refunded by an admin). A seller order with nothing left
// is canceled and its shipping refunded too.
func (s *Service) shrinkSellerOrder(ctx context.Context, tx *database.Tx, so SellerOrder, oversoldCents int64) (
	SellerOrder, error,
) {
	subtotal := max(so.SubtotalCents-oversoldCents, 0)
	refund := so.SubtotalCents - subtotal

	split := SplitOf(subtotal, so.ShippingCents, s.Config.FeeBPS)
	status := so.Status

	if subtotal == 0 {
		refund += so.ShippingCents
		split = Split{}
		status = SellerCanceled
	}

	if _, err := dbx.Exec(
		ctx,
		tx,
		`UPDATE seller_orders SET subtotal_cents = ?, shipping_cents = ?, fee_cents = ?,
		payout_cents = ?, status = ?, updated_at = ? WHERE id = ?`,
		split.SubtotalCents,
		split.ShippingCents,
		split.FeeCents,
		split.PayoutCents,
		status,
		s.now(),
		so.ID,
	); err != nil {
		return so, fmt.Errorf("shrink seller order %d: %w", so.ID, err)
	}

	if _, err := WriteEntry(ctx, tx, Entry{
		Type: EntryOversellRefund, OrderID: so.OrderID, SellerOrderID: so.ID, ShopID: so.ShopID, AmountCents: -refund,
		DedupeKey: EntryOversellRefund + ":" + strconv.FormatInt(so.ID, 10),
	}, s.now()); err != nil {
		return so, err
	}

	s.log().
		WarnContext(ctx, "order oversold", "event", "payments.oversold", "seller_order", so.ID, "refund_cents", refund)

	so.SubtotalCents, so.ShippingCents, so.FeeCents, so.PayoutCents, so.Status =
		split.SubtotalCents, split.ShippingCents, split.FeeCents, split.PayoutCents, status

	return so, nil
}
