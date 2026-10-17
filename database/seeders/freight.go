package seeders

import (
	"context"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/shipping"
)

// FreightRates writes the simulated freight table (weight band × region).
// It is reference data, safe to run on every deploy.
type FreightRates struct{}

// Seed implements vukem.Seeder.
func (FreightRates) Seed(ctx context.Context, db *database.DB) error {
	return shipping.SeedRates(ctx, db)
}
