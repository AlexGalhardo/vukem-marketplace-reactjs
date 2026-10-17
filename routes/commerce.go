package routes

import (
	"context"
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/cache"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/cart"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/checkout"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers/commerce"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments/abacatepay"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments/stripe"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/platform"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/shipping"
)

// Commerce registers the cart, address book, freight, checkout, provider
// webhooks, seller payouts and admin payouts.
func Commerce(deps *platform.Deps) func(r *router.Router, app *vukem.App) error {
	return func(r *router.Router, app *vukem.App) error {
		return commerceRoutes(r, app, deps)
	}
}

func commerceRoutes(r *router.Router, app *vukem.App, deps *platform.Deps) error {
	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	pay, err := NewPayments(app)
	if err != nil {
		return err
	}

	for _, p := range pay.Config.Problems {
		app.Log.Warn("payments provider disabled", "reason", p)
	}

	db, err := app.DB()
	if err != nil {
		return err
	}

	c, err := app.Cache()
	if err != nil {
		return err
	}

	var viaOpts []shipping.ViaCEPOption
	if app.Kind == vukem.KindTest {
		if base := app.Source.Get("VIACEP_BASE_URL"); base != "" {
			viaOpts = append(viaOpts, shipping.WithViaCEPBaseURL(base))
		}
	}

	carts := cart.Service{DB: db, Products: cart.SQLProducts{DB: db}, Now: app.Now}
	addresses := shipping.Addresses{DB: db, Now: app.Now}
	freight := shipping.Freight{DB: db}

	h := &commerce.Handlers{
		Accounts:  acc,
		Cart:      carts,
		Addresses: addresses,
		ViaCEP:    shipping.NewViaCEP(appCache{c}, viaOpts...),
		Freight:   freight,
		Checkout: checkout.Service{
			DB: db, Cart: carts, Addresses: addresses, Freight: freight, FeeBPS: pay.Config.FeeBPS, Now: app.Now,
		},
		Payments:   pay,
		Production: app.Config.IsProduction(),
	}

	// Providers are servers: their webhooks are answered before the
	// session and CSRF middleware and authenticated by signature.
	r.UseServer(pay.WebhookMiddleware)
	r.Use(h.ShareCartCount)

	required := auth.Required(acc.Web)
	byIP := func(name string, n int) router.Middleware {
		return ratelimit.Middleware(deps.Limiter(name, n, time.Minute), ratelimit.ByIP)
	}

	r.Get("/freight", h.FreightQuote).Name("freight.quote").Use(byIP("freight", 60))
	r.Get("/cep/{cep}", h.LookupCEP).Name("cep.lookup").Use(required, byIP("cep", 30))

	r.Get("/cart", h.ShowCart).Name("cart.show").Use(required)
	r.Post("/cart/items", h.AddItem).Name("cart.add").Use(required, byIP("cart", 60))
	r.Patch("/cart/items/{id}", h.UpdateItem).Name("cart.update").Use(required, byIP("cart", 60))
	r.Delete("/cart/items/{id}", h.RemoveItem).Name("cart.remove").Use(required)

	r.Get("/account/addresses", h.ShowAddresses).Name("addresses.index").Use(required)
	r.Post("/account/addresses", h.StoreAddress).Name("addresses.store").Use(required)
	r.Put("/account/addresses/{id}", h.UpdateAddress).Name("addresses.update").Use(required)
	r.Delete("/account/addresses/{id}", h.DestroyAddress).Name("addresses.destroy").Use(required)
	r.Post("/account/addresses/{id}/default", h.DefaultAddress).Name("addresses.default").Use(required)

	verified := []router.Middleware{required, h.RequireVerified}
	place := []router.Middleware{required, h.RequireVerified, byIP("checkout", 10)}

	r.Get("/checkout", h.StartCheckout).Name("checkout.start").Use(verified...)
	r.Get("/checkout/address", h.ShowAddressStep).Name("checkout.address").Use(verified...)
	r.Post("/checkout/address", h.SaveAddressStep).Use(verified...)
	r.Get("/checkout/payment", h.ShowPaymentStep).Name("checkout.payment").Use(verified...)
	r.Post("/checkout", h.Place).Name("checkout.place").Use(place...)
	r.Get("/checkout/pix/{order}", h.ShowPix).Name("checkout.pix").Use(verified...)
	r.Get("/checkout/pix/{order}/status", h.PixStatus).Name("checkout.pix.status").Use(verified...)
	r.Post("/checkout/pix/{order}/simulate", h.SimulatePix).Name("checkout.pix.simulate").Use(place...)
	r.Get("/checkout/success", h.ShowSuccess).Name("checkout.success").Use(verified...)

	r.Get("/seller/payouts", h.ShowSellerPayouts).Name("seller.payouts").Use(verified...)
	r.Post("/seller/payouts", h.RequestPayout).Name("seller.payouts.request").Use(verified...)
	r.Post("/seller/payouts/stripe", h.OnboardStripe).Name("seller.payouts.stripe").Use(verified...)
	r.Post("/seller/orders/{id}/refund", h.RefundSellerOrder).Name("seller.orders.refund").Use(verified...)

	r.Group("/admin/payouts", func(g *router.Group) {
		g.Use(acc.RequireAdmin)
		g.Get("", h.ShowAdminPayouts).Name("admin.payouts")
		g.Post("/{id}/paid", h.MarkPayoutPaid).Name("admin.payouts.paid")
		g.Post("/{id}/reject", h.RejectPayout).Name("admin.payouts.reject")
	})

	return nil
}

// NewPayments builds the payments service from the environment: Stripe and
// AbacatePay clients only when configured. STRIPE_API_BASE is honoured
// only in tests, so the key never leaves for another host in production.
func NewPayments(app *vukem.App) (*payments.Service, error) {
	cfg, err := payments.LoadConfig(app.Source.Get, app.Config.IsProduction(), app.Kind == vukem.KindTest)
	if err != nil {
		return nil, err
	}

	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	c, err := app.Cache()
	if err != nil {
		return nil, err
	}

	svc := &payments.Service{DB: db, Config: cfg, Cache: appCache{c}, Log: app.Log, Now: app.Now}

	if cfg.CardEnabled() {
		if svc.Stripe, err = stripe.New(cfg.StripeSecretKey, cfg.StripeAPIBase); err != nil {
			return nil, err
		}
	}

	if cfg.PixEnabled() {
		if svc.Pix, err = abacatepay.New(cfg.AbacatePayAPIKey, cfg.AbacatePayBaseURL); err != nil {
			return nil, err
		}
	}

	return svc, nil
}

// appCache adapts the app cache to the byte-oriented Cache interfaces.
type appCache struct{ c *cache.Cache }

func (a appCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return cache.Get[[]byte](ctx, a.c, key)
}

func (a appCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return a.c.Put(ctx, key, value, ttl)
}
