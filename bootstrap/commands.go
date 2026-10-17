package bootstrap

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/s3"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
)

// Commands lists the application commands; vukem make:command appends here.
func Commands() []vukem.Command {
	return []vukem.Command{makeAdmin{}, referenceSync{}, mediaBucket{}, &demoCatalog{}}
}

var errEmailRequired = errors.New("usage: users:make-admin <email>")

// makeAdmin promotes an existing account to admin. It is the only way to
// create admins (no web or API path); the admin must then enable 2FA.
type makeAdmin struct{}

func (makeAdmin) Name() string { return "users:make-admin" }

func (makeAdmin) Description() string { return "Grant the admin role to an existing user by email" }

func (makeAdmin) Flags(*flag.FlagSet) {}

func (makeAdmin) Run(ctx context.Context, app *vukem.App, args []string) error {
	if len(args) != 1 {
		return errEmailRequired
	}

	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	if err := acc.MakeAdmin(ctx, args[0]); err != nil {
		return err
	}

	_, err = fmt.Fprintf(app.Stdout(), "%s is now an admin; 2FA is required before using /admin\n", args[0])

	return err
}

// referenceSync upserts the reference data the app needs in every
// environment (categories, freight table). Unlike db:seed it never creates
// demo accounts, so the production image runs it on every start.
type referenceSync struct{}

func (referenceSync) Name() string { return "reference:sync" }

func (referenceSync) Description() string {
	return "Upsert reference data (categories, freight table); safe in production"
}

func (referenceSync) Flags(*flag.FlagSet) {}

func (referenceSync) Run(ctx context.Context, app *vukem.App, _ []string) error {
	db, err := app.DB()
	if err != nil {
		return err
	}

	for _, s := range []vukem.Seeder{seeders.Categories{}, seeders.FreightRates{}} {
		if err := s.Seed(ctx, db); err != nil {
			return err
		}
	}

	return nil
}

// mediaBucket creates the local S3 bucket (RustFS in compose and CI). It
// refuses production, where the bucket comes with the platform.
type mediaBucket struct{}

func (mediaBucket) Name() string { return "media:bucket" }

func (mediaBucket) Description() string {
	return "Create the local S3_BUCKET (development and CI only)"
}

func (mediaBucket) Flags(*flag.FlagSet) {}

func (mediaBucket) Run(ctx context.Context, app *vukem.App, _ []string) error {
	cfg, err := config.Load[s3.Config](app.Source)
	if err != nil {
		return err
	}

	return catalog.CreateBucket(ctx, cfg, app.Config.Env)
}

// demoCatalog seeds the demo catalog (100 sellers, 2000 products) on its
// own. db:seed runs it outside production; in production it needs
// --allow-production, and it never creates a known-password account.
type demoCatalog struct {
	allowProduction, fresh *bool
}

func (*demoCatalog) Name() string { return "demo:catalog" }

func (*demoCatalog) Description() string {
	return "Seed the demo catalog (100 sellers, 2000 products); production needs --allow-production"
}

func (d *demoCatalog) Flags(fs *flag.FlagSet) {
	d.allowProduction = fs.Bool("allow-production", false, "allow seeding when APP_ENV=production")
	d.fresh = fs.Bool("fresh", false, "delete the demo sellers (shops, products) first")
}

func (d *demoCatalog) Run(ctx context.Context, app *vukem.App, _ []string) error {
	s := seeders.DemoCatalog{AllowProduction: *d.allowProduction}

	if *d.fresh {
		if app.Config.IsProduction() && !*d.allowProduction {
			return seeders.ErrDemoCatalogInProduction
		}

		db, err := app.DB()
		if err != nil {
			return err
		}

		if err := s.Purge(ctx, db); err != nil {
			return err
		}
	}

	res, err := s.RunApp(ctx, app)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(app.Stdout(), "Demo catalog: %d sellers and %d products created in %s.\n",
		res.Sellers, res.Products, res.Elapsed.Round(time.Millisecond))

	return err
}
