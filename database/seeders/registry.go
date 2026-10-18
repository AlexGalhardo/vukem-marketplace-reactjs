// Package seeders fills the database with demo data: vukem db:seed.
package seeders

import (
	"context"

	vukem "github.com/AlexGalhardo/vukem"
)

// All lists the seeders run by vukem db:seed, in order; make:seeder appends here.
func All() []vukem.AppSeeder {
	return []vukem.AppSeeder{
		Users{},
		db("Categories", Categories{}),
		DemoShop{},
		DemoCatalog{},
		db("FreightRates", FreightRates{}),
		// After DemoCatalog: conversations between its demo sellers and
		// the demo buyers.
		DemoChat{},
	}
}

// db runs a database-only seeder in the ordered AppSeeder list.
func db(name string, s vukem.Seeder) vukem.AppSeeder {
	return vukem.NamedAppSeeder(name, func(ctx context.Context, app *vukem.App) error {
		d, err := app.DB()
		if err != nil {
			return err
		}

		return s.Seed(ctx, d)
	})
}
