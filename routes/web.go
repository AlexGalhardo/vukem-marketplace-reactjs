// Package routes declares the HTTP routes.
package routes

import (
	"cmp"
	"net/http"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/middleware"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/controllers"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/resources"
)

// Web registers the browser routes and the probes. vukem route:list shows them.
func Web() func(r *router.Router, app *vukem.App) error {
	return func(r *router.Router, app *vukem.App) error {
		if err := vukem.DefaultMiddleware(app, r); err != nil {
			return err
		}

		// Opens the database, which registers vukem's "db" health check.
		if _, err := app.DB(); err != nil {
			return err
		}

		probes := controllers.Probes{Ready: app.ReadyHandler(), Log: app.Log, Version: app.Source.Get("APP_VERSION")}
		r.UseServer(probes.Middleware)
		r.UseServer(middleware.StaticFS(resources.Public(), middleware.StaticConfig{Prefix: "/"}))
		r.Use(app.Inertia.Middleware, shareProps(app))
		r.Services().ErrorPage = errorPage(app)

		// "/" (home) is registered by the catalog routes.
		r.Post("/locale", controllers.SetLocale(app.I18n, app.Config.Env != config.Development)).Name("locale.update")

		return nil
	}
}

// shareProps adds the props every page receives (usePage().props).
//
// The pages import the JSON catalogs at build time (typed keys, no
// per-visit payload), so only locale and locales are shared; vukem's
// inertia.I18n would also ship the messages as a prop.
func shareProps(app *vukem.App) func(http.Handler) http.Handler {
	locales := app.I18n.Locales()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			inertia.Share(ctx, "appName", app.Config.Name)
			inertia.Share(ctx, "locale", i18n.Locale(ctx))
			inertia.Share(ctx, "locales", locales)
			next.ServeHTTP(w, r)
		})
	}
}

// errorPage renders the Inertia "Error" page for Inertia visits (vukem's
// own hook) and for full page loads, which vukem leaves as a raw HTML
// status page; unmatched routes (no Inertia middleware) keep that page.
//
// workaround(vukem): inertia.ErrorPage only handles Inertia visits.
func errorPage(app *vukem.App) web.ErrorPageFunc {
	return func(c *web.Ctx, e web.ErrorPage) bool {
		if app.Inertia.ErrorPage(c, e) {
			return true
		}

		if e.Status < 400 || e.Status > 599 {
			return false
		}

		props := inertia.Props{"status": e.Status, "title": e.Title, "message": cmp.Or(e.Detail, e.Title)}

		return inertia.RenderStatus(c, e.Status, "Error", props) == nil
	}
}
