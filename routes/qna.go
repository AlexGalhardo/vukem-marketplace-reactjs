package routes

import (
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers/questions"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/platform"
)

// Questions registers product questions: asking on the product page
// (verified users, 5 questions per 10 minutes each), the seller inbox and
// admin moderation (accounts.RequireAdmin, like routes.Admin). The
// storefront's product page reads its props from the returned handlers.
func Questions(
	r *router.Router,
	app *vukem.App,
	deps *platform.Deps,
	acc *accounts.Accounts,
) (*questions.Handlers, error) {
	h, err := questions.FromApp(app, acc, deps.Realtime)
	if err != nil {
		return nil, err
	}

	limiters, err := app.RateLimiters()
	if err != nil {
		return nil, err
	}

	askLimit := ratelimit.Middleware(limiters("qna-ask", 5, 10*time.Minute), questions.AskThrottleKey)
	verified := []router.Middleware{acc.RequireVerified}

	r.Post("/p/{slug}/questions", questions.Localized(h.Ask)).Name("catalog.product.questions.store").
		Use(acc.RequireVerified, askLimit)

	r.Group("/seller/questions", func(g *router.Group) {
		g.Use(verified...)
		g.Get("", h.SellerIndex).Name("seller.questions")
		g.Post("/{id}/answer", questions.Localized(h.SellerAnswer)).Name("seller.questions.answer")
	})

	r.Group("/admin/questions", func(g *router.Group) {
		g.Use(acc.RequireAdmin, acc.RequireVerified)
		g.Get("", h.AdminIndex).Name("admin.questions")
		g.Post("/{id}/hide", h.AdminHide).Name("admin.questions.hide")
		g.Post("/{id}/unhide", h.AdminUnhide).Name("admin.questions.unhide")
	})

	return h, nil
}
