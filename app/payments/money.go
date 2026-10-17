// Package payments owns the money rules: the platform fee, the append-only
// ledger, provider webhook events, card payments (Stripe Connect Express,
// separate charges and transfers), PIX (AbacatePay) and seller payouts.
// Money is int64 cents (BRL); see .claude/rules/money.md.
package payments

import "errors"

// DefaultFeeBPS is the platform fee in basis points (10%).
const DefaultFeeBPS = 1000

// maxBPS is 100%.
const maxBPS = 10_000

// ErrInvalidFeeBPS is returned for a fee outside 0..10000 basis points.
var ErrInvalidFeeBPS = errors.New("payments: fee bps must be between 0 and 10000")

// Fee returns round(subtotal × bps / 10000), rounding half up, in integers.
// The fee applies to items only, never to shipping. Negative subtotals
// yield 0.
func Fee(subtotalCents, bps int64) int64 {
	if subtotalCents <= 0 || bps <= 0 {
		return 0
	}

	return (subtotalCents*bps + maxBPS/2) / maxBPS
}

// Payout is what the seller receives for one seller order:
// subtotal + shipping − fee.
func Payout(subtotalCents, shippingCents, feeCents int64) int64 {
	return subtotalCents + shippingCents - feeCents
}

// Split is the money of one seller order.
type Split struct {
	SubtotalCents int64
	ShippingCents int64
	FeeCents      int64
	PayoutCents   int64
}

// SplitOf computes the fee and payout of a seller order.
func SplitOf(subtotalCents, shippingCents, bps int64) Split {
	fee := Fee(subtotalCents, bps)

	return Split{
		SubtotalCents: subtotalCents,
		ShippingCents: shippingCents,
		FeeCents:      fee,
		PayoutCents:   Payout(subtotalCents, shippingCents, fee),
	}
}

// ValidateFeeBPS checks a configured fee.
func ValidateFeeBPS(bps int64) error {
	if bps < 0 || bps > maxBPS {
		return ErrInvalidFeeBPS
	}

	return nil
}
