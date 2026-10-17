package payments

import (
	"context"
	"time"
)

// StripeAPI is the slice of Stripe the marketplace uses: Connect Express
// onboarding, Checkout Sessions, transfers, refunds and reversals. Every
// POST takes a deterministic idempotency key. app/payments/stripe
// implements it over HTTP; tests use fakes.
type StripeAPI interface {
	CreateExpressAccount(ctx context.Context, p ExpressAccountParams, idemKey string) (StripeAccount, error)
	GetAccount(ctx context.Context, id string) (StripeAccount, error)
	CreateAccountLink(ctx context.Context, accountID, refreshURL, returnURL string) (string, error)
	CreateCheckoutSession(ctx context.Context, p CheckoutSessionParams, idemKey string) (CheckoutSession, error)
	GetPaymentIntent(ctx context.Context, id string) (PaymentIntent, error)
	CreateTransfer(ctx context.Context, p TransferParams, idemKey string) (Transfer, error)
	CreateRefund(ctx context.Context, p RefundParams, idemKey string) (Refund, error)
	CreateTransferReversal(ctx context.Context, transferID string, amountCents int64, idemKey string) (Reversal, error)
}

// ExpressAccountParams creates a Connect Express account.
type ExpressAccountParams struct {
	Email   string
	Country string
	ShopID  int64
}

// StripeAccount is a connected account. Ready means it can receive
// transfers: readiness is the transfers capability, not charges_enabled.
type StripeAccount struct {
	ID    string
	Ready bool
}

// LineItem is one Checkout line, priced by the server.
type LineItem struct {
	Name      string
	UnitCents int64
	Quantity  int
}

// CheckoutSessionParams creates a payment-mode Checkout Session on the
// platform account (separate charges and transfers).
type CheckoutSessionParams struct {
	OrderPublicID string
	CustomerEmail string
	LineItems     []LineItem
	SuccessURL    string
	CancelURL     string
	Currency      string
}

// CheckoutSession is the part of a Checkout Session the app reads.
type CheckoutSession struct {
	ID  string
	URL string
}

// PaymentIntent is the part of a PaymentIntent the app reads.
type PaymentIntent struct {
	ID           string
	LatestCharge string
}

// TransferParams sends a seller's share to their Express account.
type TransferParams struct {
	AmountCents       int64
	Currency          string
	Destination       string
	TransferGroup     string
	SourceTransaction string
	SellerOrderID     int64
}

// Transfer is a created transfer.
type Transfer struct{ ID string }

// RefundParams refunds part of a payment back to the buyer.
type RefundParams struct {
	PaymentIntent string
	AmountCents   int64
	SellerOrderID int64
	Reason        string
}

// Refund is a created refund.
type Refund struct{ ID string }

// Reversal is a created transfer reversal.
type Reversal struct{ ID string }

// PixAPI is AbacatePay's Checkout Transparente for PIX.
// app/payments/abacatepay implements it.
type PixAPI interface {
	CreatePixCharge(ctx context.Context, p PixChargeParams) (PixCharge, error)
	SimulatePayment(ctx context.Context, chargeID string) error
}

// PixChargeParams creates a PIX charge.
type PixChargeParams struct {
	AmountCents int64
	// ExternalID is the order public id (AbacatePay's idempotency key).
	ExternalID  string
	Description string
	ExpiresIn   time.Duration
}

// PixCharge is a created PIX charge.
type PixCharge struct {
	ID     string
	Status string
	// BRCode is the copy-and-paste code.
	BRCode string
	// QRImage is a data:image/png;base64 URI of the QR code.
	QRImage   string
	ExpiresAt time.Time
}
