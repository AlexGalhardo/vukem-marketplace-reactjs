package seeders

import (
	"context"
	"fmt"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/chat"
)

// Demo account domains: demo buyers come from the demo users seeder, demo
// sellers from DemoCatalog (DemoSellerDomain). Only these accounts get
// demo conversations; real users are never touched.
const (
	demoBuyerDomain  = "compradores.example.com"
	demoSellerDomain = "vendedores.example.com"
)

// demoChatThreads is how many demo conversations are seeded at most.
const demoChatThreads = 6

// demoChatScripts are the seeded dialogues (buyer first, then alternating).
var demoChatScripts = [][]string{
	{
		"Hi! Is this product still available?",
		"Hi! It is, we have it ready to ship.",
		"Great, I'll finish the purchase today.",
	},
	{
		"Good afternoon, what is the delivery time to Belo Horizonte?",
		"Good afternoon! It usually arrives in 4 to 6 business days.",
		"Perfect, thank you!",
		"Does it come with a manufacturer's warranty?",
	},
	{"Hi, does it come with an invoice?", "Hi! Yes, all orders ship with an invoice."},
	{"Hi! Does it work with 220V?", "Hi! Yes, it is bivolt (110V and 220V).", "Great, thanks for the quick answer."},
	{
		"Do you have it in another color?",
		"We have black and gray, just choose it in the listing.",
		"I'll go with gray then.",
	},
	{"Can I exchange it if it doesn't fit?", "You can, you have 7 days to request the exchange through the platform."},
}

// DemoChat seeds a few conversations between demo buyers
// (@compradores.example.com) and demo sellers (@vendedores.example.com).
// It runs after DemoCatalog, skips quietly when either side is missing,
// never runs in production and is idempotent (InsertDemo leaves threads
// that already have messages alone).
type DemoChat struct {
	// Env overrides the app's environment (tests); empty uses the app's.
	Env config.Env
	// Now anchors the message times; nil uses time.Now.
	Now func() time.Time
}

// Name implements the db:seed name hook.
func (DemoChat) Name() string { return "DemoChat" }

// SeedApp implements vukem.AppSeeder.
func (d DemoChat) SeedApp(ctx context.Context, app *vukem.App) error {
	env := d.Env
	if env == "" {
		env = app.Config.Env
	}

	if env == config.Production {
		app.Log.InfoContext(ctx, "demo chat skipped in production")
		return nil
	}

	db, err := app.DB()
	if err != nil {
		return err
	}

	n, err := d.Seed(ctx, db)
	if err != nil {
		return err
	}

	app.Log.InfoContext(ctx, "demo chat seeded", "conversations", n)

	return nil
}

// Seed writes the demo conversations and returns how many were created.
func (d DemoChat) Seed(ctx context.Context, db *database.DB) (int, error) {
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}

	buyers, err := demoIDs(ctx, db, `SELECT id FROM users WHERE email LIKE ? ORDER BY id LIMIT ?`,
		"%@"+demoBuyerDomain, demoChatThreads)
	if err != nil {
		return 0, err
	}

	// One published product per demo shop, the lowest id: deterministic.
	products, err := demoIDs(ctx, db, `SELECT MIN(p.id) FROM products p
		JOIN shops s ON s.id = p.shop_id JOIN users u ON u.id = s.owner_id
		WHERE u.email LIKE ? AND p.status = 'published' AND s.status = 'active'
		GROUP BY s.id ORDER BY MIN(p.id) LIMIT ?`, "%@"+demoSellerDomain, demoChatThreads)
	if err != nil {
		return 0, err
	}

	threads := min(len(buyers), len(products), len(demoChatScripts))
	created := 0

	err = db.Tx(ctx, func(tx *database.Tx) error {
		for i := range threads {
			script := demoChatScripts[i]
			start := now().UTC().Add(-time.Duration(threads-i) * 3 * time.Hour).Truncate(time.Second)
			lines := make([]chat.DemoLine, len(script))

			for j, body := range script {
				lines[j] = chat.DemoLine{
					FromBuyer: j%2 == 0,
					Body:      body,
					At:        start.Add(time.Duration(j) * 7 * time.Minute),
				}
			}

			ok, err := chat.InsertDemo(ctx, tx, buyers[i], products[i], lines)
			if err != nil {
				return err
			}

			if ok {
				created++
			}
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("seed demo chat: %w", err)
	}

	return created, nil
}

func demoIDs(ctx context.Context, db *database.DB, q string, args ...any) ([]int64, error) {
	q, err := database.RebindChecked(db.Dialect(), q)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("seed demo chat: %w", err)
	}
	defer rows.Close()

	var out []int64

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("seed demo chat: %w", err)
		}

		out = append(out, id)
	}

	return out, rows.Err()
}
