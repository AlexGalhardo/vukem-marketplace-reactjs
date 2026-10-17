package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// ErrNotRefundable is returned for a seller order that is not paid,
// shipped or delivered (pending, canceled or already refunded).
var ErrNotRefundable = errs.NewValidation().Add("refund", "checkout.errors.not_refundable")

// RefundSellerOrder refunds one seller order in full (subtotal + shipping),
// started by the seller who owns it; anyone else gets 404. Card: reverse
// the transfer (key reversal-<id>), then refund the buyer (key
// refund-<id>). PIX: debit the seller's balance and log the refund owed to
// the buyer (paid out by an admin). A repeated request changes nothing.
func (s *Service) RefundSellerOrder(ctx context.Context, ownerID, sellerOrderID int64) error {
	so, err := s.ownedSellerOrder(ctx, ownerID, sellerOrderID)
	if err != nil {
		return err
	}

	if so.Status != SellerPaid && so.Status != SellerShipped && so.Status != SellerDelivered {
		return ErrNotRefundable
	}

	o, err := s.OrderByID(ctx, so.OrderID)
	if err != nil {
		return err
	}

	id := strconv.FormatInt(so.ID, 10)
	refundCents := so.SubtotalCents + so.ShippingCents

	var refundRef, reversalRef string

	if o.Method == MethodCard {
		if s.Stripe == nil {
			return ErrCardDisabled
		}

		if so.StripeTransferID != "" {
			rv, err := s.Stripe.CreateTransferReversal(ctx, so.StripeTransferID, so.PayoutCents, "reversal-"+id)
			if err != nil {
				return fmt.Errorf("reverse transfer of seller order %d: %w", so.ID, err)
			}

			reversalRef = rv.ID
		}

		rf, err := s.Stripe.CreateRefund(ctx, RefundParams{
			PaymentIntent: o.PaymentIntentID, AmountCents: refundCents, SellerOrderID: so.ID, Reason: "seller_refund",
		}, "refund-"+id)
		if err != nil {
			return fmt.Errorf("refund seller order %d: %w", so.ID, err)
		}

		refundRef = rf.ID
	}

	return s.DB.Tx(ctx, func(tx *database.Tx) error {
		now := s.now()

		n, err := dbx.Exec(ctx, tx, `UPDATE seller_orders SET status = ?, updated_at = ?
			WHERE id = ? AND status IN (?, ?, ?)`, SellerRefunded, now, so.ID, SellerPaid, SellerShipped, SellerDelivered)
		if err != nil {
			return fmt.Errorf("mark seller order %d refunded: %w", so.ID, err)
		}

		if n == 0 {
			return ErrNotRefundable // a concurrent request won
		}

		entries := []Entry{}

		if o.Method == MethodCard {
			entries = append(entries, Entry{
				Type: EntryRefund, OrderID: o.ID, SellerOrderID: so.ID, ShopID: so.ShopID, AmountCents: -refundCents,
				Provider: ProviderStripe, ProviderRef: refundRef, DedupeKey: "refund-" + id,
			})

			if reversalRef != "" {
				entries = append(entries, Entry{
					Type: EntryTransferReversed, OrderID: o.ID, SellerOrderID: so.ID, ShopID: so.ShopID,
					AmountCents: -so.PayoutCents, Provider: ProviderStripe, ProviderRef: reversalRef,
					DedupeKey: "reversal-" + id,
				})
			}
		} else {
			entries = append(entries,
				Entry{
					Type: EntrySellerDebit, OrderID: o.ID, SellerOrderID: so.ID, ShopID: so.ShopID,
					AmountCents: -so.PayoutCents, DedupeKey: EntrySellerDebit + ":" + id,
				},
				Entry{
					Type: EntryRefundDue, OrderID: o.ID, SellerOrderID: so.ID, ShopID: so.ShopID,
					AmountCents: -refundCents, DedupeKey: EntryRefundDue + ":" + id,
				})
		}

		for _, e := range entries {
			if _, err := WriteEntry(ctx, tx, e, now); err != nil {
				return err
			}
		}

		return s.updateOrderRefundStatus(ctx, tx, o.ID)
	})
}

// updateOrderRefundStatus sets refunded when no seller order is left to
// serve, partially_refunded otherwise.
func (s *Service) updateOrderRefundStatus(ctx context.Context, tx *database.Tx, orderID int64) error {
	var open int
	if err := dbx.QueryRow(ctx, tx, `SELECT COUNT(*) FROM seller_orders WHERE order_id = ? AND status NOT IN (?, ?)`,
		orderID, SellerRefunded, SellerCanceled).Scan(&open); err != nil {
		return err
	}

	status := OrderPartiallyRefunded
	if open == 0 {
		status = OrderRefunded
	}

	_, err := dbx.Exec(ctx, tx, `UPDATE orders SET status = ?, updated_at = ? WHERE id = ? AND status IN (?, ?)`,
		status, s.now(), orderID, OrderPaid, OrderPartiallyRefunded)

	return err
}

func (s *Service) ownedSellerOrder(ctx context.Context, ownerID, id int64) (SellerOrder, error) {
	so, err := scanSellerOrder(dbx.QueryRow(ctx, s.DB, sellerOrderSelect+` WHERE so.id = ? AND s.owner_id = ?`,
		id, ownerID))
	if errors.Is(err, sql.ErrNoRows) {
		return SellerOrder{}, orm.ErrNotFound
	}

	if err != nil {
		return SellerOrder{}, fmt.Errorf("seller order %d: %w", id, err)
	}

	return so, nil
}
