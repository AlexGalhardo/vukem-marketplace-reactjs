package routes

import (
	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
)

// Admin registers the admin area. Every route is behind
// accounts.RequireAdmin (admin role and confirmed 2FA) and a verified email.
func Admin(r *router.Router, app *vukem.App) error {
	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	r.Group("/admin", func(g *router.Group) {
		g.Use(acc.RequireAdmin, acc.RequireVerified)
		g.Get("", func(c *web.Ctx) error { return inertia.Render(c, "Admin/Dashboard", nil) }).Name("admin.dashboard")
	})

	return nil
}
