package routes

import (
	"context"
	"net/http"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
)

// hasShop reports whether userID owns a shop (any status).
func hasShop(ctx context.Context, db database.Executor, userID int64) (bool, error) {
	q, err := database.RebindChecked(db.Dialect(), `SELECT COUNT(*) FROM shops WHERE owner_id = ?`)
	if err != nil {
		return false, err
	}

	var n int
	if err := db.QueryRowContext(ctx, q, userID).Scan(&n); err != nil {
		return false, err
	}

	return n > 0, nil
}

// requireShop sends signed-in users without a shop to /sell with a flash
// message, for every seller-only route (any method, nothing changes). It
// runs after auth.Required; a database error fails closed with a 500.
func requireShop(acc *accounts.Accounts, db database.Executor) router.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := web.NewCtx(w, r, web.ServicesFrom(r.Context()))

			u, err := acc.Web.User(c)
			if err != nil {
				c.HandleError(err)
				return
			}

			ok, err := hasShop(r.Context(), db, u.ID)
			if err != nil {
				c.HandleError(err)
				return
			}

			if !ok {
				inertia.Flash(c, "success", i18n.T(r.Context(), "seller.flash.open_shop_first"))

				if err := c.Redirect("/sell"); err != nil {
					c.HandleError(err)
				}

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
