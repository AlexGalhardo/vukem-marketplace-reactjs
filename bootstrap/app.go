// Package bootstrap wires the application together. There is no hidden
// registration: everything the app uses is listed here.
package bootstrap

import (
	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/postgres"
	"github.com/AlexGalhardo/vukem/inertia"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/jobs"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/listeners"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/platform"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/migrations"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/resources"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/routes"
)

// New builds the application; cmd/app and the tests call it.
func New(opts ...vukem.Option) (*vukem.App, error) {
	deps := &platform.Deps{}
	jobBinding := &jobs.Binding{}

	base := []vukem.Option{
		vukem.WithDatabaseDrivers(postgres.Driver()),
		vukem.WithProviders(append(Providers(), platform.MongoProvider{Deps: deps})...),
		vukem.WithMigrations(migrations.All()),
		vukem.WithAppSeeders(seeders.All()...),
		vukem.WithListeners(listeners.All()...),
		vukem.WithJobs(jobs.All(jobBinding)...),
		vukem.WithSchedule(routes.Schedule),
		vukem.WithViews(resources.Views()),
		vukem.WithVite(resources.Public()),
		vukem.WithTranslations(resources.Lang()),
		vukem.WithInertia(inertia.Config{}),
		vukem.WithFlags(flags.Registered()...),
	}
	base = append(base, platform.RedisOptions(deps)...)

	for _, fn := range routes.All(deps) {
		base = append(base, vukem.WithRoutes(fn))
	}

	for _, fn := range routes.AfterBoot(deps) {
		base = append(base, vukem.WithRoutesAfterBoot(fn))
	}

	app, err := vukem.New(append(base, opts...)...)
	if err != nil {
		if deps.Redis != nil {
			_ = deps.Redis.Close()
		}

		return nil, err
	}

	jobBinding.Bind(app)
	deps.Bind(app)

	app.AddCommand(Commands()...)

	return app, nil
}
