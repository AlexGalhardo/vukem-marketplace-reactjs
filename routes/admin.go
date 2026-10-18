package routes

import (
	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/controllers/admin"
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

		ff := &admin.Flags{Accounts: acc, Flags: acc.Flags}
		g.Get("/flags", ff.Show).Name("admin.flags")
		g.Post("/flags", ff.Update).Name("admin.flags.update")
	})

	return nil
}
