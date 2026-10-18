package routes

import (
	"context"
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/controllers"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/platform"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/webhooks"
)

// Ops registers the contact form, the seller webhook pages and the
// realtime event stream.
func Ops(deps *platform.Deps) func(r *router.Router, app *vukem.App) error {
	return func(r *router.Router, app *vukem.App) error {
		db, err := app.DB()
		if err != nil {
			return err
		}

		acc, err := accounts.New(app)
		if err != nil {
			return err
		}

		limiters, err := app.RateLimiters()
		if err != nil {
			return err
		}

		required := auth.Required(acc.Web)

		contact := controllers.Contact{DB: db, Queue: app.Queue, Log: app.Log}
		// 5 messages per 10 minutes per IP: the form is public.
		contactLimit := ratelimit.Middleware(limiters("contact", 5, 10*time.Minute), ratelimit.ByIP)

		r.Get("/contact", contact.Show).Name("contact")
		r.Post("/contact", contact.Store).Name("contact.store").Use(contactLimit)

		wh, err := webhooks.New(app)
		if err != nil {
			return err
		}

		seller := controllers.SellerWebhooks{Service: wh}
		shopOwner := []router.Middleware{required, requireShop(acc, db)}
		r.Get("/seller/webhooks", seller.Index).Name("seller.webhooks").Use(shopOwner...)
		r.Post("/seller/webhooks", seller.Store).Name("seller.webhooks.store").Use(shopOwner...)
		r.Get("/seller/webhooks/{id}", seller.Show).Name("seller.webhooks.show").Use(shopOwner...)
		r.Delete("/seller/webhooks/{id}", seller.Destroy).Name("seller.webhooks.destroy").Use(shopOwner...)

		events := controllers.Events{Hub: deps.Realtime}
		// Reconnects are cheap for EventSource; this only stops loops.
		streamLimit := ratelimit.Middleware(limiters("events", 30, time.Minute), ratelimit.ByIP)
		r.Get("/events", events.Stream).Name("events").Use(required, streamLimit)

		if app.Kind == vukem.KindWeb {
			app.AddRunner("realtime-relay", deps.Realtime)
		}

		app.OnShutdown(func(context.Context) error { deps.Realtime.Close(); return nil })

		return nil
	}
}
