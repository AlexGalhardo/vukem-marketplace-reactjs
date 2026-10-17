//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/redis"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/vukemtest"
	"github.com/AlexGalhardo/vukem/webhook"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/maintenance"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/realtime"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/webhooks"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

// TestRealtimeRelaysAcrossInstances publishes on one hub (a worker) and
// receives on another (a web instance) through Redis pub/sub.
func TestRealtimeRelaysAcrossInstances(t *testing.T) {
	url := env(t, "TEST_REDIS_URL")

	open := func() *redis.Client {
		c, err := redis.Open(t.Context(), url)
		if err != nil {
			t.Fatal(err)
		}

		t.Cleanup(func() { _ = c.Close() })

		return c
	}

	channel := "test:realtime:" + rand.Text()
	worker := realtime.NewHub(open(), channel, nil)
	web := realtime.NewHub(open(), channel, nil)
	t.Cleanup(web.Close)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	go func() { _ = web.Run(ctx) }()

	alice, stopA := web.Subscribe(ctx, 1)
	bob, stopB := web.Subscribe(ctx, 2)
	t.Cleanup(stopA)
	t.Cleanup(stopB)

	// The relay subscribes asynchronously: publish until the first event
	// for bob arrives.
	deadline := time.After(10 * time.Second)

	for ready := false; !ready; {
		if err := worker.Publish(t.Context(), 2, realtime.Notifications, "ready"); err != nil {
			t.Fatal(err)
		}

		select {
		case <-bob:
			ready = true
		case <-time.After(50 * time.Millisecond):
		case <-deadline:
			t.Fatal("relay never delivered")
		}
	}

	if err := worker.Publish(t.Context(), 1, realtime.Orders, map[string]string{"status": "paid"}); err != nil {
		t.Fatal(err)
	}

	select {
	case ev := <-alice:
		if ev.Event != realtime.Orders || ev.Data != `{"status":"paid"}` {
			t.Fatalf("alice got %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("alice's event not relayed")
	}

	select {
	case ev := <-bob:
		if ev.Data != `"ready"` {
			t.Fatalf("bob received another user's event: %+v", ev)
		}
	case <-time.After(200 * time.Millisecond):
	}
}

// TestOpsSQLOnPostgres runs the pruning and webhook delivery-log SQL on
// Postgres (the unit tests use SQLite).
func TestOpsSQLOnPostgres(t *testing.T) {
	dbURL := env(t, "TEST_DATABASE_URL")

	fake := queue.NewFake()
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{
		Env:        map[string]string{"DATABASE_URL": dbURL},
		AppOptions: []vukem.Option{vukem.WithQueueDriver(fake)},
	})

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	p := maintenance.Pruner{DB: db, ResetTTL: time.Hour}
	for name, fn := range map[string]func(context.Context) error{
		"tokens": p.Tokens, "provider events": p.ProviderEvents, "webhook deliveries": p.WebhookDeliveries,
	} {
		if err := fn(t.Context()); err != nil {
			t.Errorf("prune %s: %v", name, err)
		}
	}

	tag := rand.Text()[:10]
	u := models.User{Name: "Seller", Email: "ops-" + tag + "@example.com", Password: "x", Role: models.RoleUser}

	if err := orm.Create(t.Context(), db, &u); err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(
		t.Context(),
		"INSERT INTO shops (owner_id, slug, name, created_at, updated_at) VALUES ($1, $2, $3, now(), now())",
		u.ID,
		"ops-"+tag,
		"Ops shop",
	); err != nil {
		t.Fatal(err)
	}

	svc, err := webhooks.New(app.App)
	if err != nil {
		t.Fatal(err)
	}

	shopID, err := svc.ShopOf(t.Context(), u.ID)
	if err != nil {
		t.Fatal(err)
	}

	ep, secret, err := svc.Create(
		t.Context(),
		shopID,
		"https://hooks.example.com/"+tag,
		[]string{webhooks.EventOrderPaid},
	)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.Find(t.Context(), shopID, ep.ID)
	if err != nil || got.Secret.V != secret {
		t.Fatalf("secret does not round-trip on Postgres: %v", err)
	}

	if err := svc.Dispatch(t.Context(), shopID, webhooks.EventOrderPaid, map[string]any{"order_id": tag}); err != nil {
		t.Fatal(err)
	}

	msgs := fake.Messages()
	if len(msgs) != 1 || msgs[0].Kind != webhook.DeliverKind {
		t.Fatalf("queued %+v", msgs)
	}

	rows, err := svc.Recent(t.Context(), ep.ID, 10)
	if err != nil || len(rows) != 1 || rows[0].Status != webhooks.StatusPending {
		t.Fatalf("delivery rows %+v, err %v", rows, err)
	}

	if err := svc.Delete(t.Context(), shopID, ep.ID); err != nil {
		t.Fatal(fmt.Errorf("delete endpoint: %w", err))
	}
}
