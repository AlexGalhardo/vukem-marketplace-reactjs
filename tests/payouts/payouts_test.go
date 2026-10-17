// Package payouts tests seller balances (ledger sums), payout requests and
// the admin's manual PIX payout record.
package payouts

import (
	"strconv"
	"sync"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/flagtest"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/tests/commercetest"
)

// paidPixSale makes seller's shop earn payout 9900 + 1590 − 990 = 10500
// through a paid PIX order and returns the shop id.
func paidPixSale(t *testing.T, e *commercetest.Env, seller models.User) int64 {
	t.Helper()

	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{})
	slug, _ := e.Product(t, shop, commercetest.Product{PriceCents: 9900, Stock: 5, WeightGrams: 100})
	buyer := e.User(t, true)
	address := e.Address(t, buyer.ID)

	e.Checkout(t, buyer, address, "pix", map[string]int{slug: 1})

	charge := e.String(t, `SELECT abacatepay_charge_id FROM orders WHERE buyer_id = ?`, buyer.ID)
	e.PostPix(t, commercetest.PixWebhookSecret, payments.AbacatePayPublicKey,
		commercetest.PixPaidEvent("evt_"+charge, charge, 9900+1590)).AssertOK()

	return shop
}

const wantBalance = 10_500

func TestPayoutRequestAndBalanceFromLedger(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{Pix: true})
	seller := e.User(t, true)
	shop := paidPixSale(t, e, seller)

	c := e.Client(t, seller).WithHeader("Referer", "/seller/payouts")
	c.Get("/seller/payouts").AssertOK().AssertInertia("Seller/Payouts").
		PropEquals("summary.balanceCents", float64(wantBalance)).
		PropEquals("summary.availableCents", float64(wantBalance)).
		PropEquals("stripe.enabled", false)

	// Users without a shop: sent to /sell.
	e.Client(t, e.User(t, true)).Get("/seller/payouts").AssertRedirect("/sell")
	flagtest.RequireEmailVerification(t, e.App)
	e.Client(t, e.User(t, false)).Get("/seller/payouts").AssertRedirect("/email/verify")

	tests := []struct {
		amount int64
		ok     bool
	}{
		{wantBalance + 1, false},
		{500, false}, // under the minimum
		{-1000, false},
		{6000, true},
		{4501, false}, // 6000 already requested: 4500 left
		{4500, true},
		{1000, false},
	}

	for _, tt := range tests {
		c.PostJSON("/seller/payouts", map[string]any{"amountCents": tt.amount}).AssertRedirect("/seller/payouts")
		page := c.Get("/seller/payouts").AssertInertia("Seller/Payouts")

		if tt.ok {
			page.MissingProp("errors.amountCents")
		} else {
			page.HasProp("errors.amountCents")
		}
	}

	if n := e.Int64(t, `SELECT SUM(amount_cents) FROM payouts WHERE shop_id = ?`, shop); n != wantBalance {
		t.Errorf("requested = %d, want %d", n, wantBalance)
	}

	// Requests do not move the ledger: only paid payouts do.
	c.Get("/seller/payouts").AssertInertia("Seller/Payouts").
		PropEquals("summary.balanceCents", float64(wantBalance)).
		PropEquals("summary.availableCents", float64(0))
}

func TestPayoutConcurrentRequestsCannotOverdraw(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{Pix: true})
	seller := e.User(t, true)
	shop := paidPixSale(t, e, seller)

	var wg sync.WaitGroup

	for range 5 {
		wg.Go(func() {
			e.Client(t, seller).WithHeader("Referer", "/seller/payouts").
				PostJSON("/seller/payouts", map[string]any{"amountCents": 6000})
		})
	}

	wg.Wait()

	if n := e.Int64(t, `SELECT COUNT(*) FROM payouts WHERE shop_id = ?`, shop); n != 1 {
		t.Errorf("payouts = %d, want 1 (only one fits the balance)", n)
	}
}

func TestPayoutAdminMarksPaid(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{Pix: true})
	seller := e.User(t, true)
	shop := paidPixSale(t, e, seller)

	s := e.Client(t, seller).WithHeader("Referer", "/seller/payouts")
	s.PostJSON("/seller/payouts", map[string]any{"amountCents": 4000}).AssertRedirect("/seller/payouts")
	s.PostJSON("/seller/payouts", map[string]any{"amountCents": 3000}).AssertRedirect("/seller/payouts")

	first := e.Int64(t, `SELECT MIN(id) FROM payouts WHERE shop_id = ?`, shop)
	second := e.Int64(t, `SELECT MAX(id) FROM payouts WHERE shop_id = ?`, shop)
	paidPath := "/admin/payouts/" + strconv.FormatInt(first, 10) + "/paid"
	e2e := "E12345678202610171200abcdefghijk"

	// Not an admin: the area does not exist.
	s.Get("/admin/payouts").AssertNotFound()
	s.PostJSON(paidPath, map[string]any{"endToEndId": e2e}).AssertNotFound()
	e.App.Client(t).Get("/admin/payouts").AssertNotFound()

	admin := e.Admin(t)
	a := e.Client(t, admin).WithHeader("Referer", "/admin/payouts")
	a.Get("/admin/payouts").AssertOK().AssertInertia("Admin/Payouts").HasProp("pending", 2)

	a.PostJSON(paidPath, map[string]any{"endToEndId": "not-an-e2e"}).AssertRedirect("/admin/payouts")
	a.Get("/admin/payouts").AssertInertia("Admin/Payouts").HasProp("errors.endToEndId")

	a.PostJSON(paidPath, map[string]any{"endToEndId": e2e}).AssertRedirect("/admin/payouts")
	vukemtest.AssertDatabaseHas(t, e.App, "payouts", map[string]any{
		"id": first, "status": "paid", "pix_end_to_end_id": e2e, "paid_by": admin.ID,
	})
	vukemtest.AssertDatabaseHas(t, e.App, "ledger_entries", map[string]any{
		"type": "payout.paid", "shop_id": shop, "amount_cents": -4000, "provider_ref": e2e,
	})

	// Paying twice, or reusing the end-to-end id, is refused.
	a.PostJSON(paidPath, map[string]any{"endToEndId": "E12345678202610171200zzzzzzzzzzz"}).
		AssertRedirect("/admin/payouts")
	a.Get("/admin/payouts").AssertInertia("Admin/Payouts").HasProp("errors.payout")
	a.PostJSON("/admin/payouts/"+strconv.FormatInt(second, 10)+"/paid", map[string]any{"endToEndId": e2e}).
		AssertRedirect("/admin/payouts")
	a.Get("/admin/payouts").AssertInertia("Admin/Payouts").HasProp("errors.endToEndId")
	vukemtest.AssertDatabaseHas(t, e.App, "payouts", map[string]any{"id": second, "status": "requested"})

	a.PostJSON("/admin/payouts/999999/paid", map[string]any{"endToEndId": e2e}).AssertNotFound()

	// Rejecting frees the amount again.
	a.PostJSON("/admin/payouts/"+strconv.FormatInt(second, 10)+"/reject", map[string]any{"reason": "Wrong PIX key"}).
		AssertRedirect("/admin/payouts")

	s.Get("/seller/payouts").AssertInertia("Seller/Payouts").
		PropEquals("summary.balanceCents", float64(wantBalance-4000)).
		PropEquals("summary.availableCents", float64(wantBalance-4000))

	if b := e.Int64(t, `SELECT SUM(amount_cents) FROM ledger_entries WHERE shop_id = ? AND type IN (?, ?, ?)`,
		shop, payments.EntrySellerCredit, payments.EntrySellerDebit, payments.EntryPayoutPaid); b != wantBalance-4000 {
		t.Errorf("ledger balance = %d, want %d", b, wantBalance-4000)
	}
}
