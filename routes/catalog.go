package routes

import (
	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/controllers/seller"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/controllers/storefront"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/jobs"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/platform"
)

// Catalog registers the storefront (home, search, category, product, shop,
// images) and the seller area (open a shop, dashboard, products). Selling
// needs a verified email. Registered with WithRoutesAfterBoot, so the
// MongoDB client the platform provider opens in Boot exists.
func Catalog(deps *platform.Deps) func(r *router.Router, app *vukem.App) error {
	return func(r *router.Router, app *vukem.App) error {
		svc, err := catalog.FromApp(app, deps.Mongo)
		if err != nil {
			return err
		}

		q, err := app.Queue()
		if err != nil {
			return err
		}

		if err := q.Register(jobs.Catalog(svc)...); err != nil {
			return err
		}

		acc, err := accounts.New(app)
		if err != nil {
			return err
		}

		rv, err := reviewHandlers(app, acc)
		if err != nil {
			return err
		}

		pub := storefront.New(svc, acc).WithReviews(rv.ProductBlock)

		if pub.Questions, err = Questions(r, app, deps, acc); err != nil {
			return err
		}

		r.Get("/", pub.Home).Name("home")
		r.Get("/search", pub.Search).Name("catalog.search")
		r.Get("/search/suggest", pub.Suggest).Name("catalog.suggest")
		r.Get("/c/{slug}", pub.Category).Name("catalog.category")
		r.Get("/p/{slug}", pub.Product).Name("catalog.product")
		r.Get("/s/{slug}", pub.Shop).Name("shop.show")
		r.Get("/media/{kind}/{id}/{file}", pub.Media).Name("catalog.media")

		h := seller.New(svc, acc)
		verified := []router.Middleware{acc.RequireVerified}

		r.Get("/sell", h.ShowOpen).Name("shop.create").Use(verified...)
		r.Post("/sell", h.Open).Name("shop.store").Use(verified...)

		db, err := app.DB()
		if err != nil {
			return err
		}

		r.Group("/seller", func(g *router.Group) {
			g.Use(append(verified, requireShop(acc, db))...)
			g.Get("", h.Dashboard).Name("seller.dashboard")
			g.Get("/settings", h.Settings).Name("seller.settings")
			g.Put("/settings", h.UpdateSettings).Name("seller.settings.update")
			g.Post("/settings/logo", h.UploadLogo).Name("seller.settings.logo")
			g.Get("/products", h.Products).Name("seller.products")
			g.Get("/products/new", h.NewProduct).Name("seller.products.create")
			g.Post("/products", h.CreateProduct).Name("seller.products.store")
			g.Get("/products/{id}/edit", h.EditProduct).Name("seller.products.edit")
			g.Put("/products/{id}", h.UpdateProduct).Name("seller.products.update")
			g.Post("/products/{id}/status", h.SetStatus).Name("seller.products.status")
			g.Post("/products/{id}/images", h.UploadImages).Name("seller.products.images.store")
			g.Put("/products/{id}/images/order", h.ReorderImages).Name("seller.products.images.order")
			g.Delete("/products/{id}/images/{image}", h.DeleteImage).Name("seller.products.images.destroy")
		})

		return nil
	}
}
