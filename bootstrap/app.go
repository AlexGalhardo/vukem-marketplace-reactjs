// Package bootstrap wires the application together. There is no hidden
// registration: everything the app uses is listed here.
package bootstrap

import (
	"context"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/postgres"
	"github.com/AlexGalhardo/vukem/inertia"

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

	redisOpts, err := platform.RedisOptions(context.Background(), deps)
	if err != nil {
		return nil, err
	}

	base := []vukem.Option{
		vukem.WithDatabaseDrivers(postgres.Driver()),
		vukem.WithProviders(append(Providers(), platform.MongoProvider{Deps: deps})...),
		vukem.WithMigrations(migrations.All()),
		vukem.WithSeeders(seeders.All()...),
		vukem.WithListeners(listeners.All()...),
		vukem.WithJobs(jobs.All(jobBinding)...),
		vukem.WithSchedule(routes.Schedule),
		vukem.WithViews(resources.Views()),
		vukem.WithVite(resources.Public()),
		vukem.WithTranslations(resources.Lang()),
		vukem.WithInertia(inertia.Config{}),
	}
	base = append(base, redisOpts...)

	for _, fn := range routes.All(deps) {
		base = append(base, vukem.WithRoutes(fn))
	}

	app, err := vukem.New(append(base, opts...)...)
	if err != nil {
		if deps.Redis != nil {
			_ = deps.Redis.Close()
		}

		return nil, err
	}

	jobBinding.Bind(app)

	if deps.Redis != nil {
		app.OnShutdown(func(context.Context) error { return deps.Redis.Close() })
	}

	app.AddCommand(Commands()...)

	return app, nil
}
