package bootstrap

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"

	vukem "github.com/AlexGalhardo/vukem"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/reviews"
)

// reviewsRecalculate rebuilds products.rating_avg_x100 and rating_count
// from the visible reviews: of the given product ids, or of every product.
type reviewsRecalculate struct{}

func (reviewsRecalculate) Name() string { return "reviews:recalculate" }

func (reviewsRecalculate) Description() string {
	return "Rebuild product ratings from their visible reviews (all products, or the given ids)"
}

func (reviewsRecalculate) Flags(*flag.FlagSet) {}

func (reviewsRecalculate) Run(ctx context.Context, app *vukem.App, args []string) error {
	ids := make([]int64, 0, len(args))

	for _, a := range args {
		id, err := strconv.ParseInt(a, 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("reviews:recalculate: %q is not a product id", a)
		}

		ids = append(ids, id)
	}

	db, err := app.DB()
	if err != nil {
		return err
	}

	if err := reviews.Recalculate(ctx, db, ids...); err != nil {
		return err
	}

	_, err = fmt.Fprintln(app.Stdout(), "Product ratings recalculated.")

	return err
}

var (
	errPaidOrderUsage      = errors.New("usage: dev:paid-order <buyer-email> <product-slug>")
	errPaidOrderProduction = errors.New("dev:paid-order refuses APP_ENV=production")
)

// devPaidOrder records a paid order without a payment provider, so a
// local or e2e buyer can review a product. Never in production.
type devPaidOrder struct{}

func (devPaidOrder) Name() string { return "dev:paid-order" }

func (devPaidOrder) Description() string {
	return "Record a paid order of a product for a buyer (development and e2e only)"
}

func (devPaidOrder) Flags(*flag.FlagSet) {}

func (devPaidOrder) Run(ctx context.Context, app *vukem.App, args []string) error {
	if app.Config.IsProduction() {
		return errPaidOrderProduction
	}

	if len(args) != 2 {
		return errPaidOrderUsage
	}

	db, err := app.DB()
	if err != nil {
		return err
	}

	item, err := reviews.DemoPurchase(ctx, db, args[0], args[1], app.Now())
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(app.Stdout(), item)

	return err
}
