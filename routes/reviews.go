package routes

import (
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	reviewsctl "github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers/reviews"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/reviews"
)

// reviewHandlers builds the review controllers on the app database.
func reviewHandlers(app *vukem.App, acc *accounts.Accounts) (*reviewsctl.Handlers, error) {
	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	return &reviewsctl.Handlers{Reviews: &reviews.Service{DB: db, Now: app.Now}, Accounts: acc}, nil
}

// Reviews registers the review form, helpful votes, the shop's reply and
// admin moderation. Writing, voting and replying need a session, are
// rate limited per IP and answer 404 while the reviews flag is off; the
// product page block is added by the Catalog routes.
func Reviews(r *router.Router, app *vukem.App) error {
	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	h, err := reviewHandlers(app, acc)
	if err != nil {
		return err
	}

	limiters, err := app.RateLimiters()
	if err != nil {
		return err
	}

	write := func(name string, n int) []router.Middleware {
		return []router.Middleware{
			auth.Required(acc.Web), acc.Flags.Require(flags.Reviews),
			ratelimit.Middleware(limiters("reviews-"+name, n, time.Minute), ratelimit.ByIP),
		}
	}

	r.Get("/order-items/{item}/review", h.ShowForm).Name("reviews.create").Use(write("form", 60)...)
	r.Post("/order-items/{item}/review", h.Store).Name("reviews.store").Use(write("store", 20)...)
	r.Post("/reviews/{id}/helpful", h.Vote).Name("reviews.helpful").Use(write("vote", 30)...)
	r.Post("/reviews/{id}/reply", h.Reply).Name("reviews.reply").Use(write("reply", 20)...)

	r.Group("/admin/reviews", func(g *router.Group) {
		g.Use(acc.RequireAdmin, acc.RequireVerified)
		g.Get("", h.AdminIndex).Name("admin.reviews")
		g.Post("/{id}/hide", h.Hide).Name("admin.reviews.hide")
		g.Post("/{id}/unhide", h.Unhide).Name("admin.reviews.unhide")
	})

	return nil
}
