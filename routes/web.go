// Package routes declares the HTTP routes.
package routes

import (
	"net/http"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/middleware"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/platform"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/resources"
)

// Web registers the browser routes and the probes. vukem route:list shows them.
func Web(deps *platform.Deps) func(r *router.Router, app *vukem.App) error {
	return func(r *router.Router, app *vukem.App) error {
		if err := vukem.DefaultMiddleware(app, r); err != nil {
			return err
		}

		db, err := app.DB()
		if err != nil {
			return err
		}

		deps.AddCheck("database", db.Ping)

		probes := controllers.Probes{Deps: deps, Log: app.Log, Version: app.Source.Get("APP_VERSION")}
		r.UseServer(probes.Middleware)
		r.UseServer(middleware.StaticFS(resources.Public(), middleware.StaticConfig{Prefix: "/"}))
		r.Use(app.Inertia.Middleware, shareProps(app))

		r.Get("/", controllers.Home).Name("home")
		r.Post("/locale", controllers.SetLocale(app.I18n, app.Config.Env != config.Development)).Name("locale.update")

		return nil
	}
}

// shareProps adds the props every page receives (usePage().props).
//
// workaround(vukem): Inertia has no built-in locale sharing, so the active
// locale is shared here and the pages import the same JSON catalogs.
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
