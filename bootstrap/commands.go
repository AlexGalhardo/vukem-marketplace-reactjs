package bootstrap

import (
	"context"
	"errors"
	"flag"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/s3"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
)

// Commands lists the application commands; vukem make:command appends here.
func Commands() []vukem.Command {
	return []vukem.Command{makeAdmin{}, referenceSync{}, mediaBucket{}}
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

	// workaround(vukem): commands cannot reach the app's stdout writer
	// (WithStdout is private state), so the confirmation goes to the log.
	app.Log.InfoContext(ctx, "user is now an admin; 2FA is required before using /admin", "email", args[0])

	return nil
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
