package routes

import (
	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/platform"
)

// All lists the route files, in order.
func All(deps *platform.Deps) []func(r *router.Router, app *vukem.App) error {
	return []func(r *router.Router, app *vukem.App) error{
		Web(),
		Auth(deps),
		Social,
		TwoFactor(deps),
		Admin,
		API,
		Commerce(deps),
		Ops(deps),
		Reviews,
	}
}

// AfterBoot lists the route files that need clients opened in a
// provider's Boot (vukem.WithRoutesAfterBoot).
func AfterBoot(deps *platform.Deps) []func(r *router.Router, app *vukem.App) error {
	return []func(r *router.Router, app *vukem.App) error{
		Catalog(deps),
	}
}
