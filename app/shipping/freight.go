package shipping

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// Rate is one row of the freight table: up to MaxWeightGrams to Region
// costs PriceCents and takes Days business days.
type Rate struct {
	Region         Region
	MaxWeightGrams int
	PriceCents     int64
	Days           int
}

// Quote is the price of shipping one seller order.
type Quote struct {
	Region      Region `json:"region"`
	WeightGrams int    `json:"weightGrams"`
	PriceCents  int64  `json:"priceCents"`
	Days        int    `json:"days"`
}

// ExtraKiloCents is charged per started kilogram above the heaviest band.
const ExtraKiloCents = 500

// extraDays is added when a parcel is above the heaviest band.
const extraDays = 2

// ErrNoRates is returned when the freight table has no row for a region
// (the table was not seeded: run vukem db:seed).
var ErrNoRates = errors.New("shipping: freight table has no rates for the region")

// ErrInvalidWeight is returned for a negative weight.
var ErrInvalidWeight = errors.New("shipping: invalid weight")

// DefaultRates is the simulated table seeded into freight_rates: weight
// bands of 300 g, 1, 2, 5, 10 and 30 kg per destination region. Prices are
// illustrative (no carrier contract, PLAN D10).
func DefaultRates() []Rate {
	bands := []int{300, 1000, 2000, 5000, 10_000, 30_000}
	table := []struct {
		region Region
		days   int
		prices [6]int64
	}{
		{Southeast, 3, [6]int64{1590, 1990, 2490, 3490, 4990, 8990}},
		{South, 4, [6]int64{1790, 2290, 2790, 3890, 5590, 9990}},
		{CenterWest, 5, [6]int64{1990, 2490, 2990, 4290, 6190, 10990}},
		{Northeast, 7, [6]int64{2290, 2790, 3390, 4790, 6990, 12990}},
		{North, 9, [6]int64{2590, 3190, 3790, 5390, 7790, 14990}},
	}

	rates := make([]Rate, 0, len(bands)*len(table))

	for _, row := range table {
		for i, w := range bands {
			rates = append(
				rates,
				Rate{Region: row.region, MaxWeightGrams: w, PriceCents: row.prices[i], Days: row.days},
			)
		}
	}

	return rates
}

// QuoteFrom prices a parcel of weightGrams to region with rates: the
// lightest band that fits; above the heaviest band, its price plus
// ExtraKiloCents per started extra kilogram. Weight 0 (unknown) uses the
// lightest band.
func QuoteFrom(rates []Rate, region Region, weightGrams int) (Quote, error) {
	if weightGrams < 0 {
		return Quote{}, ErrInvalidWeight
	}

	var bands []Rate

	for _, r := range rates {
		if r.Region == region {
			bands = append(bands, r)
		}
	}

	if len(bands) == 0 {
		return Quote{}, ErrNoRates
	}

	slices.SortFunc(bands, func(a, b Rate) int { return cmp.Compare(a.MaxWeightGrams, b.MaxWeightGrams) })

	q := Quote{Region: region, WeightGrams: weightGrams}

	for _, r := range bands {
		if weightGrams <= r.MaxWeightGrams {
			q.PriceCents, q.Days = r.PriceCents, r.Days
			return q, nil
		}
	}

	top := bands[len(bands)-1]
	extraKilos := (weightGrams - top.MaxWeightGrams + 999) / 1000
	q.PriceCents = top.PriceCents + int64(extraKilos)*ExtraKiloCents
	q.Days = top.Days + extraDays

	return q, nil
}

// Freight reads the freight table from the database.
type Freight struct {
	DB database.Executor
}

// Rates loads every row of freight_rates.
func (f Freight) Rates(ctx context.Context) ([]Rate, error) {
	rows, err := f.DB.QueryContext(ctx, `SELECT region, max_weight_grams, price_cents, days FROM freight_rates`)
	if err != nil {
		return nil, fmt.Errorf("load freight rates: %w", err)
	}
	defer rows.Close()

	var rates []Rate

	for rows.Next() {
		var r Rate
		if err := rows.Scan(&r.Region, &r.MaxWeightGrams, &r.PriceCents, &r.Days); err != nil {
			return nil, fmt.Errorf("scan freight rate: %w", err)
		}

		rates = append(rates, r)
	}

	return rates, rows.Err()
}

// Quote prices a parcel of weightGrams to the state uf.
func (f Freight) Quote(ctx context.Context, uf string, weightGrams int) (Quote, error) {
	region, err := RegionOf(uf)
	if err != nil {
		return Quote{}, err
	}

	rates, err := f.Rates(ctx)
	if err != nil {
		return Quote{}, err
	}

	return QuoteFrom(rates, region, weightGrams)
}

// SeedRates writes DefaultRates into freight_rates, replacing the prices
// of rows that already exist (safe to run on every deploy).
func SeedRates(ctx context.Context, ex database.Executor) error {
	for _, r := range DefaultRates() {
		_, err := dbx.Exec(ctx, ex,
			`INSERT INTO freight_rates (region, max_weight_grams, price_cents, days) VALUES (?, ?, ?, ?)
			ON CONFLICT (region, max_weight_grams) DO UPDATE SET price_cents = excluded.price_cents, days = excluded.days`,
			string(r.Region), r.MaxWeightGrams, r.PriceCents, r.Days)
		if err != nil {
			return fmt.Errorf("seed freight rate %s/%d: %w", r.Region, r.MaxWeightGrams, err)
		}
	}

	return nil
}
