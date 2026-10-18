package realtime_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/controllers"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/realtime"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/authtest"
)

// The hub that /events uses lives in bootstrap's platform deps; the test
// mounts the same controller and guard on its own hub so it can publish.
const streamPath = "/events-under-test"

type event struct{ Name, Data string }

func boot(t *testing.T) (*vukemtest.App, *realtime.Hub, *httptest.Server) {
	t.Helper()

	hub := realtime.NewHub(nil, "", nil)
	t.Cleanup(hub.Close)

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{AppOptions: []vukem.Option{
		vukem.WithRoutes(func(r *router.Router, app *vukem.App) error {
			acc, err := accounts.New(app)
			if err != nil {
				return err
			}

			r.Get(streamPath, controllers.Events{Hub: hub}.Stream).Use(auth.Required(acc.Web))

			return nil
		}),
	}})

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	for _, email := range []string{"alice@example.com", "bob@example.com"} {
		u := models.User{Name: "U", Email: email, Password: "x", Role: models.RoleUser}
		if err := orm.Create(t.Context(), db, &u); err != nil {
			t.Fatal(err)
		}
	}

	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)

	return app, hub, srv
}

// open connects as userID (0: anonymous) and returns the status, the
// content type and the stream's events. The body closes at cleanup.
func open(
	t *testing.T,
	app *vukemtest.App,
	srv *httptest.Server,
	path string,
	userID int64,
) (int, string, <-chan event) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Accept", "text/event-stream")

	if userID != 0 {
		for _, ck := range authtest.SessionCookies(t, app, userID) {
			req.AddCookie(ck)
		}
	}

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = resp.Body.Close() })

	events := make(chan event, 16)

	go func() {
		defer close(events)

		var ev event

		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()

			switch {
			case strings.HasPrefix(line, "event:"):
				ev.Name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				ev.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			case line == "" && ev.Data != "":
				events <- ev
				ev = event{}
			}
		}
	}()

	return resp.StatusCode, resp.Header.Get("Content-Type"), events
}

func next(t *testing.T, events <-chan event) event {
	t.Helper()

	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("stream closed")
		}

		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("no event within 5s")
	}

	return event{}
}

func TestEventStreamDeliversOnlyTheUsersOwnEvents(t *testing.T) {
	t.Parallel()

	app, hub, srv := boot(t)

	statusA, typeA, alice := open(t, app, srv, streamPath, 1)
	statusB, typeB, bob := open(t, app, srv, streamPath, 2)

	for _, r := range []struct {
		status int
		ct     string
	}{{statusA, typeA}, {statusB, typeB}} {
		if r.status != http.StatusOK || !strings.HasPrefix(r.ct, "text/event-stream") {
			t.Fatalf("stream answered %d %q", r.status, r.ct)
		}
	}

	// Sync point: each user receives a marker on their own stream, which
	// proves both subscriptions exist before the real events go out.
	for _, u := range []struct {
		id     int64
		events <-chan event
	}{{1, alice}, {2, bob}} {
		deadline := time.Now().Add(5 * time.Second)

		for {
			if hub.Publish(t.Context(), u.id, realtime.Notifications, "ready") != nil {
				t.Fatal("publish failed")
			}

			select {
			case ev := <-u.events:
				if ev.Data != `"ready"` {
					t.Fatalf("user %d first event %+v, want the ready marker", u.id, ev)
				}
			case <-time.After(20 * time.Millisecond):
				if time.Now().After(deadline) {
					t.Fatalf("user %d never subscribed", u.id)
				}

				continue
			}

			break
		}
	}

	// Drain markers published before the subscription answered.
	drain := func(ch <-chan event) {
		for {
			select {
			case <-ch:
			case <-time.After(50 * time.Millisecond):
				return
			}
		}
	}
	drain(alice)
	drain(bob)

	if err := hub.Publish(
		t.Context(),
		2,
		realtime.Orders,
		map[string]string{"order": "bob-1", "status": "paid"},
	); err != nil {
		t.Fatal(err)
	}

	if err := hub.Publish(
		t.Context(),
		1,
		realtime.Orders,
		map[string]string{"order": "alice-1", "status": "shipped"},
	); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(
		event{Name: "orders", Data: `{"order":"alice-1","status":"shipped"}`},
		next(t, alice),
	); diff != "" {
		t.Errorf("alice's event (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(event{Name: "orders", Data: `{"order":"bob-1","status":"paid"}`}, next(t, bob)); diff != "" {
		t.Errorf("bob's event (-want +got):\n%s", diff)
	}

	// Nothing else (in particular not bob's event) reaches alice.
	select {
	case ev := <-alice:
		t.Fatalf("alice received another event: %+v", ev)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestEventStreamRequiresLogin(t *testing.T) {
	t.Parallel()

	app, _, srv := boot(t)

	for _, path := range []string{"/events", streamPath} {
		if status, _, _ := open(t, app, srv, path, 0); status == http.StatusOK {
			t.Errorf("%s answered 200 to an anonymous client", path)
		}
	}
}

func TestPublishRejectsUnknownChannels(t *testing.T) {
	t.Parallel()

	hub := realtime.NewHub(nil, "", nil)
	t.Cleanup(hub.Close)

	if err := hub.Publish(t.Context(), 1, "admin", "x"); err == nil {
		t.Fatal("published on an unknown channel")
	}
}
