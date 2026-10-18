package commerce

import (
	"errors"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/payments"
)

// PayoutForm is a seller's payout request.
type PayoutForm struct {
	AmountCents int64 `json:"amountCents" form:"amountCents"`
}

// PayoutFormSchema validates PayoutForm.
var PayoutFormSchema = z.Struct[PayoutForm](z.Shape{"amountCents": z.Int64().Positive()})

// MarkPaidForm records a manual PIX payout.
type MarkPaidForm struct {
	EndToEndID string `json:"endToEndId" form:"endToEndId"`
}

// MarkPaidFormSchema validates MarkPaidForm.
var MarkPaidFormSchema = z.Struct[MarkPaidForm](z.Shape{"endToEndId": z.String().Trim().Min(1).Max(64)})

// RejectForm declines a payout.
type RejectForm struct {
	Reason string `json:"reason" form:"reason"`
}

// RejectFormSchema validates RejectForm.
var RejectFormSchema = z.Struct[RejectForm](z.Shape{"reason": z.String().Trim().Min(3).Max(500)})

// ShowSellerPayouts renders the seller's balance, payouts and Stripe status.
// Users without a shop get 404.
func (h *Handlers) ShowSellerPayouts(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	shop, err := h.Payments.ShopOf(c.Context(), u.ID)
	if err != nil {
		return err
	}

	// Back from Stripe onboarding: refresh readiness now rather than wait
	// for account.updated.
	if c.Query("stripe") == "return" && shop.StripeAccountID != "" && h.Payments.Stripe != nil {
		if err := h.Payments.SyncAccount(c.Context(), shop.StripeAccountID); err != nil {
			c.Log().WarnContext(c.Context(), "stripe account sync failed", "shop", shop.ID, "err", err)
		} else if shop, err = h.Payments.ShopOf(c.Context(), u.ID); err != nil {
			return err
		}
	}

	summary, err := h.Payments.Summary(c.Context(), shop.ID)
	if err != nil {
		return err
	}

	list, err := h.Payments.ShopPayouts(c.Context(), shop.ID)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Seller/Payouts", inertia.Props{
		"shop":    shop,
		"summary": summary,
		"payouts": list,
		"stripe": map[string]bool{
			"enabled":   h.Payments.Config.CardEnabled() && h.Payments.Stripe != nil,
			"connected": shop.StripeAccountID != "",
			"ready":     shop.StripeReady,
		},
		"minPayoutCents": payments.MinPayoutCents,
	})
}

// RequestPayout asks for part of the available balance.
func (h *Handlers) RequestPayout(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	shop, err := h.Payments.ShopOf(c.Context(), u.ID)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, PayoutFormSchema)
	if err != nil {
		return err
	}

	if err := h.Payments.RequestPayout(c.Context(), shop.ID, in.AmountCents); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "checkout.payouts.flash.requested"))

	return c.Redirect("/seller/payouts")
}

// OnboardStripe sends the seller to Stripe Express onboarding.
func (h *Handlers) OnboardStripe(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	shop, err := h.Payments.ShopOf(c.Context(), u.ID)
	if err != nil {
		return err
	}

	url, err := h.Payments.Onboard(c.Context(), shop, u.Email)
	if errors.Is(err, payments.ErrCardDisabled) {
		return errs.NewValidation().Add("stripe", "checkout.payouts.errors.stripe_disabled")
	}

	if err != nil {
		c.Log().ErrorContext(c.Context(), "stripe onboarding failed", "shop", shop.ID, "err", err)
		return errs.NewValidation().Add("stripe", "checkout.payouts.errors.stripe_failed")
	}

	return inertia.Location(c, url)
}

// RefundSellerOrder refunds one of the seller's orders in full.
func (h *Handlers) RefundSellerOrder(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := paramID(c)
	if err != nil {
		return err
	}

	if err := h.Payments.RefundSellerOrder(c.Context(), u.ID, id); err != nil {
		if errors.Is(err, payments.ErrCardDisabled) {
			return errs.NewValidation().Add("refund", "checkout.errors.provider")
		}

		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "checkout.flash.refunded"))

	return c.Back("/seller/payouts")
}

// ShowAdminPayouts lists payouts waiting for a manual PIX and recent ones.
func (h *Handlers) ShowAdminPayouts(c *web.Ctx) error {
	pending, err := h.Payments.PayoutsByStatus(c.Context(), payments.PayoutRequested)
	if err != nil {
		return err
	}

	paid, err := h.Payments.PayoutsByStatus(c.Context(), payments.PayoutPaid)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Admin/Payouts", inertia.Props{"pending": pending, "paid": paid})
}

// MarkPayoutPaid records a manual PIX payout.
func (h *Handlers) MarkPayoutPaid(c *web.Ctx) error {
	admin, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := paramID(c)
	if err != nil {
		return err
	}

	if err := h.Payments.PayoutExists(c.Context(), id); err != nil {
		return err
	}

	in, err := web.Parse(c, MarkPaidFormSchema)
	if err != nil {
		return err
	}

	if err := h.Payments.MarkPayoutPaid(c.Context(), admin.ID, id, in.EndToEndID); err != nil {
		return err
	}

	web.SecurityEvent(c.Request(), "payout.paid", "payout", id, "admin", admin.ID)
	inertia.Flash(c, "success", i18n.T(c.Context(), "checkout.admin_payouts.flash.paid"))

	return c.Redirect("/admin/payouts")
}

// RejectPayout declines a payout request.
func (h *Handlers) RejectPayout(c *web.Ctx) error {
	admin, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := paramID(c)
	if err != nil {
		return err
	}

	if err := h.Payments.PayoutExists(c.Context(), id); err != nil {
		return err
	}

	in, err := web.Parse(c, RejectFormSchema)
	if err != nil {
		return err
	}

	if err := h.Payments.RejectPayout(c.Context(), id, in.Reason); err != nil {
		return err
	}

	web.SecurityEvent(c.Request(), "payout.rejected", "payout", id, "admin", admin.ID)
	inertia.Flash(c, "success", i18n.T(c.Context(), "checkout.admin_payouts.flash.rejected"))

	return c.Redirect("/admin/payouts")
}
