package maintenance_test

import (
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/maintenance"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

func setup(t *testing.T) (*database.DB, *vukemtest.FakeClock) {
	t.Helper()

	app := vukemtest.New(t, bootstrap.New)

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	return db, vukemtest.NewFakeClock(time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC))
}

func exec(t *testing.T, db *database.DB, query string, args ...any) {
	t.Helper()

	if _, err := db.ExecContext(t.Context(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func count(t *testing.T, db *database.DB, table string) int {
	t.Helper()

	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}

	return n
}

func TestPrunerTokens(t *testing.T) {
	t.Parallel()

	db, clock := setup(t)
	now := clock.Now()

	exec(t, db, "INSERT INTO password_resets (email, hash, created_at) VALUES (?, ?, ?), (?, ?, ?)",
		"old@example.com", "h1", now.Add(-2*time.Hour), "new@example.com", "h2", now.Add(-10*time.Minute))
	exec(t, db, "INSERT INTO remember_tokens (id, user_id, hash, expires_at) VALUES (?, ?, ?, ?), (?, ?, ?, ?)",
		"r1", "1", "h", now.Add(-time.Minute), "r2", "1", "h", now.Add(time.Hour))
	exec(t, db, "INSERT INTO access_tokens (id, user_id, name, hash, abilities, expires_at, created_at) VALUES "+
		"(?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?)",
		"a1", "1", "old", "h", "*", now.Add(-time.Minute), now,
		"a2", "1", "live", "h", "*", now.Add(time.Hour), now,
		"a3", "1", "forever", "h", "*", nil, now)

	p := maintenance.Pruner{DB: db, ResetTTL: time.Hour, Now: clock.Now}
	if err := p.Tokens(t.Context()); err != nil {
		t.Fatal(err)
	}

	for table, want := range map[string]int{"password_resets": 1, "remember_tokens": 1, "access_tokens": 2} {
		if got := count(t, db, table); got != want {
			t.Errorf("%s: %d rows, want %d", table, got, want)
		}
	}

	// Two hours later the remaining reset link has expired as well.
	clock.Advance(2 * time.Hour)

	if err := p.Tokens(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := count(t, db, "password_resets"); got != 0 {
		t.Errorf("password_resets after expiry: %d rows, want 0", got)
	}
}

func TestPrunerProviderEvents(t *testing.T) {
	t.Parallel()

	db, clock := setup(t)
	old := clock.Now().Add(-maintenance.ProviderEventRetention - time.Hour)

	exec(
		t,
		db,
		"INSERT INTO provider_events (provider, event_id, type, payload_digest, received_at, processed_at) VALUES "+
			"(?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)",
		"stripe",
		"evt_old",
		"x",
		"d",
		old,
		old,
		"stripe",
		"evt_unprocessed",
		"x",
		"d",
		old,
		nil,
		"stripe",
		"evt_recent",
		"x",
		"d",
		clock.Now(),
		clock.Now(),
	)

	if err := (maintenance.Pruner{DB: db, Now: clock.Now}).ProviderEvents(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := count(t, db, "provider_events"); got != 2 {
		t.Errorf("provider_events: %d rows, want 2 (unprocessed and recent kept)", got)
	}
}

func TestPrunerWebhookDeliveries(t *testing.T) {
	t.Parallel()

	db, clock := setup(t)
	now := clock.Now()
	old := now.Add(-maintenance.WebhookDeliveryRetention - time.Hour)

	exec(
		t,
		db,
		"INSERT INTO users (id, name, email, password, created_at, updated_at) VALUES (1, 'S', 's@example.com', 'x', ?, ?)",
		now,
		now,
	)
	exec(
		t,
		db,
		"INSERT INTO shops (id, owner_id, slug, name, created_at, updated_at) VALUES (1, 1, 's', 'S', ?, ?)",
		now,
		now,
	)
	exec(
		t,
		db,
		"INSERT INTO webhook_endpoints (id, shop_id, url, secret, events, created_at, updated_at) VALUES (1, 1, 'https://example.com', 'x', 'order.paid', ?, ?)",
		now,
		now,
	)
	exec(t, db, "INSERT INTO webhook_deliveries (endpoint_id, event_id, event, status, created_at, updated_at) VALUES "+
		"(1, 'e1', 'order.paid', 'delivered', ?, ?), (1, 'e2', 'order.paid', 'pending', ?, ?), (1, 'e3', 'order.paid', 'failed', ?, ?)",
		old, old, old, old, now, now)

	if err := (maintenance.Pruner{DB: db, Now: clock.Now}).WebhookDeliveries(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := count(t, db, "webhook_deliveries"); got != 2 {
		t.Errorf("webhook_deliveries: %d rows, want 2 (pending and recent kept)", got)
	}
}
