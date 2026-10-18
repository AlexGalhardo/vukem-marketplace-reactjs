package webhooks_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/vukemtest"
	"github.com/AlexGalhardo/vukem/webhook"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/webhooks"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/authtest"
)

const page = "/seller/webhooks"

// seller creates a user owning a shop and returns a logged-in client.
func seller(t *testing.T, app *vukemtest.App, n int) (*vukemtest.Client, int64) {
	t.Helper()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	u := models.User{
		Name:     "Seller",
		Email:    fmt.Sprintf("seller%d@example.com", n),
		Password: "x",
		Role:     models.RoleUser,
	}
	u.EmailVerifiedAt.Valid, u.EmailVerifiedAt.V = true, time.Now()

	if err := orm.Create(t.Context(), db, &u); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	if _, err := db.ExecContext(t.Context(),
		"INSERT INTO shops (owner_id, slug, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		u.ID, fmt.Sprintf("shop-%d", n), "Shop", now, now); err != nil {
		t.Fatal(err)
	}

	var shopID int64
	if err := db.QueryRowContext(t.Context(), "SELECT id FROM shops WHERE owner_id = ?", u.ID).
		Scan(&shopID); err != nil {
		t.Fatal(err)
	}

	return authtest.ActingAs(t, app, app.Client(t).Inertia(""), u), shopID
}

func endpoints(t *testing.T, app *vukemtest.App, shopID int64) []webhooks.Endpoint {
	t.Helper()

	db, _ := app.DB()

	list, err := orm.QueryOf[webhooks.Endpoint](db).Where("shop_id", shopID).Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return list
}

func TestSellerCreatesEndpointAndSeesSecretOnce(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	c, shopID := seller(t, app, 1)

	c.Get(page).AssertInertia("Seller/Webhooks").PropEquals("hasShop", true).HasProp("endpoints", 0)

	c.PostJSON(page, map[string]any{"url": "https://hooks.example.com/mkt", "events": []string{"order.refunded", "order.paid"}}).
		AssertRedirect(page)

	p := c.Get(page).AssertInertia("Seller/Webhooks").HasProp("endpoints", 1).
		PropEquals("endpoints.0.url", "https://hooks.example.com/mkt").
		PropEquals("endpoints.0.events", []string{"order.paid", "order.refunded"}).
		MissingProp("endpoints.0.secret")

	secret, _ := p.Prop("secret")
	if s, _ := secret.(string); !strings.HasPrefix(s, "whsec_") {
		t.Fatalf("secret = %v, want a whsec_ value right after creation", secret)
	}

	c.Get(page).AssertInertia("Seller/Webhooks").PropEquals("secret", nil)

	// Encrypted at rest: the column never holds the plain secret.
	db, _ := app.DB()

	var stored string
	if err := db.QueryRowContext(t.Context(), "SELECT secret FROM webhook_endpoints WHERE shop_id = ?", shopID).
		Scan(&stored); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(stored, secret.(string)) || strings.Contains(stored, "whsec_") {
		t.Fatal("webhook secret stored in plain text")
	}

	if got := endpoints(t, app, shopID); len(got) != 1 || got[0].Secret.V != secret {
		t.Fatalf("stored secret does not decrypt to the one shown")
	}
}

func TestEndpointURLsThatReachInternalNetworksAreRejected(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	c, shopID := seller(t, app, 1)

	urls := []string{
		"http://hooks.example.com/",                 // plain http
		"ftp://hooks.example.com/",                  // other scheme
		"https://127.0.0.1/hook",                    // loopback
		"https://10.0.0.7/hook",                     // private
		"https://192.168.1.10/hook",                 // private
		"https://169.254.169.254/latest/meta-data/", // cloud metadata
		"https://[::1]/hook",                        // IPv6 loopback
		"https://[::ffff:127.0.0.1]/hook",           // IPv4-mapped loopback
		"https://0.0.0.0/hook",                      // unspecified
		"https://localhost/hook",                    // local name
		"https://api.localhost/hook",                // *.localhost
		"https://metadata.google.internal/",         // internal suffix
		"https://intranet/hook",                     // single-label name
		"https://user:pass@hooks.example.com/",      // credentials
		"https:///no-host",                          // no host
	}

	for _, u := range urls {
		c.WithHeader("Referer", page).
			PostJSON(page, map[string]any{"url": u, "events": []string{"order.paid"}}).
			AssertRedirect(page)
		c.Get(page).AssertInertia("Seller/Webhooks").HasProp("errors.url")
	}

	c.WithHeader("Referer", page).
		PostJSON(page, map[string]any{"url": "https://hooks.example.com/", "events": []string{"order.shipped"}}).
		AssertRedirect(page)
	c.Get(page).AssertInertia("Seller/Webhooks").HasProp("errors.events")

	if got := endpoints(t, app, shopID); len(got) != 0 {
		t.Fatalf("created %d endpoints from rejected input", len(got))
	}
}

func TestAnotherSellersEndpointIsNotFound(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	owner, shopA := seller(t, app, 1)
	other, _ := seller(t, app, 2)

	owner.PostJSON(page, map[string]any{"url": "https://hooks.example.com/a", "events": []string{"order.paid"}}).
		AssertRedirect(page)
	id := endpoints(t, app, shopA)[0].ID
	target := fmt.Sprintf("%s/%d", page, id)

	other.Get(target).AssertNotFound()
	other.Delete(target).AssertNotFound()
	other.Get(page).AssertInertia("Seller/Webhooks").HasProp("endpoints", 0)

	if len(endpoints(t, app, shopA)) != 1 {
		t.Fatal("another seller deleted the endpoint")
	}

	owner.Get(target).AssertInertia("Seller/WebhookDeliveries").PropEquals("endpoint.id", id)
	owner.Delete(target).AssertRedirect(page)

	if len(endpoints(t, app, shopA)) != 0 {
		t.Fatal("owner could not delete the endpoint")
	}
}

func TestUserWithoutShopCannotCreateEndpoints(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	db, _ := app.DB()

	u := models.User{Name: "Buyer", Email: "buyer@example.com", Password: "x", Role: models.RoleUser}
	if err := orm.Create(t.Context(), db, &u); err != nil {
		t.Fatal(err)
	}

	c := authtest.ActingAs(t, app, app.Client(t).Inertia(""), u)
	c.Get(page).AssertRedirect("/sell")
	c.PostJSON(page, map[string]any{"url": "https://hooks.example.com/", "events": []string{"order.paid"}}).
		AssertRedirect("/sell")
	vukemtest.AssertDatabaseCount(t, app, "webhook_endpoints", 0)

	app.Client(t).Get(page).AssertRedirect("/login")
}

// fixture boots an app whose queue only records jobs, plus a service
// allowed to reach httptest servers on loopback (the production one is
// not).
type fixture struct {
	app      *vukemtest.App
	fake     *queue.Fake
	shopID   int64
	insecure *webhooks.Service
	secure   *webhooks.Service
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	fake := queue.NewFake()
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{AppOptions: []vukem.Option{vukem.WithQueueDriver(fake)}})
	_, shopID := seller(t, app, 1)

	insecure, err := webhooks.NewWithOptions(app.App, webhooks.Options{
		InsecureAllowHTTP: true,
		InsecureAllow:     []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
	})
	if err != nil {
		t.Fatal(err)
	}

	secure, err := webhooks.New(app.App)
	if err != nil {
		t.Fatal(err)
	}

	return &fixture{app: app, fake: fake, shopID: shopID, insecure: insecure, secure: secure}
}

// endpoint registers url for the fixture's shop, bypassing the HTTPS and
// public-address checks as only tests can.
func (f *fixture) endpoint(t *testing.T, url string, events ...string) (webhooks.Endpoint, string) {
	t.Helper()

	ep, secret, err := f.insecure.Create(t.Context(), f.shopID, url, events)
	if err != nil {
		t.Fatal(err)
	}

	return ep, secret
}

// queued returns the deliveries Dispatch queued.
func (f *fixture) queued(t *testing.T) []webhook.Delivery {
	t.Helper()

	var out []webhook.Delivery

	msgs := f.fake.Messages()
	for i := range msgs {
		m := &msgs[i]
		if m.Kind != webhook.DeliverKind {
			continue
		}

		var d webhook.Delivery
		if err := json.Unmarshal(m.Payload, &d); err != nil {
			t.Fatal(err)
		}

		out = append(out, d)
	}

	return out
}

func (f *fixture) delivery(t *testing.T, endpointID int64) webhooks.Delivery {
	t.Helper()

	db, _ := f.app.DB()

	d, err := orm.QueryOf[webhooks.Delivery](db).Where("endpoint_id", endpointID).First(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return d
}

// recorder is a receiver endpoint. With a secret it verifies signatures
// as a real receiver would (vukem webhook.Standard), answering 401 on a
// mismatch; otherwise it answers status.
type recorder struct {
	*httptest.Server

	mu      sync.Mutex
	secret  string
	status  int
	bodies  [][]byte
	headers []http.Header
}

func newRecorder(t *testing.T, status int) *recorder {
	t.Helper()

	rc := &recorder{status: status}
	rc.Server = httptest.NewServer(http.HandlerFunc(rc.serve))
	t.Cleanup(rc.Close)

	return rc
}

func (rc *recorder) serve(w http.ResponseWriter, r *http.Request) {
	rc.mu.Lock()
	secret, status := rc.secret, rc.status
	rc.mu.Unlock()

	record := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)

		rc.mu.Lock()
		rc.bodies = append(rc.bodies, b)
		rc.headers = append(rc.headers, r.Header.Clone())
		rc.mu.Unlock()

		if status == http.StatusFound {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", status)
			return
		}

		w.WriteHeader(status)
	})

	if secret == "" {
		record.ServeHTTP(w, r)
		return
	}

	verify, err := webhook.Standard(secret, webhook.StandardOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	verify(record).ServeHTTP(w, r)
}

func (rc *recorder) setSecret(s string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.secret = s
}

func (rc *recorder) received() ([][]byte, []http.Header) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return rc.bodies, rc.headers
}

func TestDispatchDeliversSignedEventsReceiversCanVerify(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	good := newRecorder(t, http.StatusOK)
	paid, secret := f.endpoint(t, good.URL+"/hook", webhooks.EventOrderPaid)
	good.setSecret(secret)

	refunds := newRecorder(t, http.StatusOK)
	f.endpoint(t, refunds.URL, webhooks.EventOrderRefunded)

	if err := f.secure.Dispatch(
		t.Context(),
		f.shopID,
		webhooks.EventOrderPaid,
		map[string]any{"order_id": "o_1", "total_cents": 1999},
	); err != nil {
		t.Fatal(err)
	}

	queued := f.queued(t)
	if len(queued) != 1 || queued[0].Endpoint != strconv.FormatInt(paid.ID, 10) {
		t.Fatalf("queued %+v, want one delivery to endpoint %d only", queued, paid.ID)
	}

	if err := f.insecure.Deliver(t.Context(), queued[0]); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	bodies, headers := good.received()
	if len(bodies) != 1 {
		t.Fatalf("receiver verified %d deliveries, want 1", len(bodies))
	}

	var env webhooks.Envelope
	if err := json.Unmarshal(bodies[0], &env); err != nil {
		t.Fatal(err)
	}

	if env.Type != webhooks.EventOrderPaid || env.ID != queued[0].ID || headers[0].Get("webhook-id") != env.ID {
		t.Fatalf("envelope %+v, webhook-id %q", env, headers[0].Get("webhook-id"))
	}

	if got := f.delivery(
		t,
		paid.ID,
	); got.Status != webhooks.StatusDelivered || got.Attempts != 1 ||
		got.ResponseCode.Int64 != http.StatusOK {
		t.Fatalf("delivery row %+v, want delivered after one 200 attempt", got)
	}

	// The same delivery checked with another secret is refused, so the
	// 200 above means the signature really verified.
	good.setSecret(mustSecret(t))

	if err := f.insecure.Deliver(t.Context(), queued[0]); err == nil {
		t.Fatal("a receiver with a different secret accepted the delivery")
	}

	if bodies, _ := good.received(); len(bodies) != 1 {
		t.Fatal("a delivery with a wrong signature reached the receiver handler")
	}

	if bodies, _ := refunds.received(); len(bodies) != 0 {
		t.Fatal("an endpoint received an event it did not subscribe to")
	}
}

func TestDispatchRejectsUnknownEvents(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	if err := f.secure.Dispatch(t.Context(), f.shopID, "order.stolen", nil); !errors.Is(err, webhooks.ErrUnknownEvent) {
		t.Fatalf("err = %v, want ErrUnknownEvent", err)
	}
}

func TestProductionSenderRefusesInternalAddresses(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	internal := newRecorder(t, http.StatusOK)

	// https, so only the address check can stop it.
	ep, _ := f.endpoint(t, "https://"+internal.Listener.Addr().String()+"/hook", webhooks.EventOrderPaid)

	if err := f.secure.Dispatch(
		t.Context(),
		f.shopID,
		webhooks.EventOrderPaid,
		map[string]any{"order_id": "o_1"},
	); err != nil {
		t.Fatal(err)
	}

	// The production sender (no InsecureAllow) must not connect to a
	// loopback address even though the endpoint row points there, as it
	// would after a hostname is rebound to 127.0.0.1.
	if err := f.secure.Deliver(t.Context(), f.queued(t)[0]); err == nil {
		t.Fatal("delivery to a loopback address succeeded")
	}

	if bodies, _ := internal.received(); len(bodies) != 0 {
		t.Fatalf("internal server received %d requests", len(bodies))
	}

	got := f.delivery(t, ep.ID)
	if got.Status != webhooks.StatusPending || got.LastError.String != webhook.AttemptBlockedAddress ||
		got.Attempts != 1 ||
		!got.NextAttemptAt.Valid {
		t.Fatalf("delivery row %+v, want pending, blocked_address, retry scheduled", got)
	}
}

func TestRedirectsAreNotFollowedAndGoneDisablesTheEndpoint(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	redirect := newRecorder(t, http.StatusFound)
	gone := newRecorder(t, http.StatusGone)

	redirectEP, _ := f.endpoint(t, redirect.URL, webhooks.EventOrderPaid)
	goneEP, _ := f.endpoint(t, gone.URL, webhooks.EventOrderPaid)

	if err := f.secure.Dispatch(
		t.Context(),
		f.shopID,
		webhooks.EventOrderPaid,
		map[string]any{"order_id": "o_1"},
	); err != nil {
		t.Fatal(err)
	}

	for _, d := range f.queued(t) {
		// 3xx and 410 are final: Deliver reports them done (no retry).
		if err := f.insecure.Deliver(t.Context(), d); err != nil {
			t.Fatalf("deliver to endpoint %s: %v", d.Endpoint, err)
		}
	}

	if r := f.delivery(t, redirectEP.ID); r.Status != webhooks.StatusFailed || r.LastError.String != "http_302" {
		t.Fatalf("redirect endpoint row %+v, want failed http_302", r)
	}

	if g := f.delivery(t, goneEP.ID); g.Status != webhooks.StatusFailed || g.ResponseCode.Int64 != http.StatusGone {
		t.Fatalf("gone endpoint row %+v, want failed 410", g)
	}

	for _, e := range endpoints(t, f.app, f.shopID) {
		if e.ID == goneEP.ID && e.Active {
			t.Fatal("endpoint answering 410 Gone is still active")
		}
	}

	// Disabled endpoints get no new events.
	if err := f.secure.Dispatch(t.Context(), f.shopID, webhooks.EventOrderPaid, nil); err != nil {
		t.Fatal(err)
	}

	if n := len(f.queued(t)); n != 3 {
		t.Fatalf("%d deliveries queued in total, want 3 (the disabled endpoint skipped)", n)
	}
}

func mustSecret(t *testing.T) string {
	t.Helper()

	s, err := webhook.NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	return s
}
