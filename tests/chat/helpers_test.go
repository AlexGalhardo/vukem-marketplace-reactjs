// Package chat_test holds the HTTP-level and WebSocket tests of the buyer
// ↔ seller chat (in-memory SQLite, broker-less broadcast).
package chat_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	cws "github.com/coder/websocket"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/chat"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/authtest"
)

// env is one booted app with a fake queue, a seller with a published
// product and helpers to act as other users.
type env struct {
	app     *vukemtest.App
	queue   *queue.Fake
	owner   models.User
	shop    catalog.ShopView
	product catalog.SellerProduct
	n       int
}

func newEnv(t *testing.T, extra ...map[string]string) *env {
	t.Helper()

	opts := vukemtest.Options{AppOptions: []vukem.Option{}}
	fake := queue.NewFake()
	opts.AppOptions = append(opts.AppOptions, vukem.WithQueueDriver(fake))

	if len(extra) > 0 {
		opts.Env = extra[0]
	}

	app := vukemtest.New(t, bootstrap.New, opts)

	if err := (seeders.Categories{}).Seed(t.Context(), db(t, app)); err != nil {
		t.Fatal(err)
	}

	e := &env{app: app, queue: fake}
	e.owner = e.user(t, "Olga Owner")
	e.shop, e.product = e.sellerWithProduct(t, e.owner, "Loja da Olga", "Liquidificador Turbo")

	return e
}

func db(t *testing.T, app *vukemtest.App) *database.DB {
	t.Helper()

	d, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	return d
}

func (e *env) exec(t *testing.T, query string, args ...any) {
	t.Helper()

	d := db(t, e.app)
	if _, err := d.ExecContext(t.Context(), database.Rebind(d.Dialect(), query), args...); err != nil {
		t.Fatal(err)
	}
}

// user creates a verified user with a unique email.
func (e *env) user(t *testing.T, name string) models.User {
	t.Helper()

	e.n++
	u := models.User{Name: name, Email: fmt.Sprintf("chat%d@example.com", e.n), Password: "x", Role: models.RoleUser}
	u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = time.Now().UTC(), true

	if err := orm.Create(t.Context(), db(t, e.app), &u); err != nil {
		t.Fatal(err)
	}

	return u
}

// admin creates an admin with confirmed two-factor authentication.
func (e *env) admin(t *testing.T) models.User {
	t.Helper()

	u := e.user(t, "Ada Admin")
	e.exec(t, `UPDATE users SET role = ?, two_factor_confirmed_at = ? WHERE id = ?`,
		models.RoleAdmin, time.Now().UTC(), u.ID)
	u.Role = models.RoleAdmin

	return u
}

func (e *env) catalog(t *testing.T) *catalog.Service {
	t.Helper()

	svc, err := catalog.FromApp(e.app.App, nil)
	if err != nil {
		t.Fatal(err)
	}

	return svc
}

// sellerWithProduct opens a shop for u with one published product.
func (e *env) sellerWithProduct(
	t *testing.T,
	u models.User,
	shopName, title string,
) (catalog.ShopView, catalog.SellerProduct) {
	t.Helper()

	svc := e.catalog(t)

	shop, err := svc.OpenShop(t.Context(), u.ID, catalog.ShopInput{Name: shopName, ShipFromUF: "SP"})
	if err != nil {
		t.Fatal(err)
	}

	p, err := svc.CreateProduct(t.Context(), u.ID, catalog.ProductInput{
		Title: title, Description: "About " + title, PriceCents: 19990, Stock: 3, WeightGrams: 500,
	})
	if err != nil {
		t.Fatal(err)
	}

	if p, err = svc.SetProductStatus(t.Context(), u.ID, p.ID, catalog.StatusPublished); err != nil {
		t.Fatal(err)
	}

	return shop, p
}

// json returns a JSON client signed in as u.
func (e *env) json(t *testing.T, u models.User) *vukemtest.Client {
	t.Helper()

	return authtest.ActingAs(t, e.app, e.app.Client(t).AcceptJSON(), u)
}

// inertia returns an Inertia client signed in as u.
func (e *env) inertia(t *testing.T, u models.User) *vukemtest.Client {
	t.Helper()

	return authtest.ActingAs(t, e.app, e.app.Client(t).Inertia(""), u)
}

// start opens buyer's conversation about the env's product and returns
// its id.
func (e *env) start(t *testing.T, buyer models.User) int64 {
	t.Helper()

	res := e.json(t, buyer).PostJSON("/messages", map[string]any{"product": e.product.Slug})
	loc := res.Header("Location")

	if res.Status() != http.StatusFound && res.Status() != http.StatusSeeOther ||
		!strings.HasPrefix(loc, "/messages/") {
		t.Fatalf("start: status %d location %q body %s", res.Status(), loc, res.Body())
	}

	id, err := strconv.ParseInt(strings.TrimPrefix(loc, "/messages/"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	return id
}

func path(id int64, suffix ...string) string {
	return "/messages/" + strconv.FormatInt(id, 10) + strings.Join(suffix, "")
}

// send posts body as u and returns the stored message id.
func (e *env) send(t *testing.T, u models.User, id int64, body string) int64 {
	t.Helper()

	res := e.json(t, u).PostJSON(path(id), map[string]any{"body": body}).AssertCreated()

	v, ok := res.JSONPath("message.id")
	if !ok {
		t.Fatalf("send: no message id in %s", res.Body())
	}

	n, err := strconv.ParseInt(fmt.Sprint(v), 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	return n
}

// service is a chat service over the app's database and cache (the same
// state the routes use), without broadcast or notifications.
func (e *env) service(t *testing.T, now func() time.Time) *chat.Service {
	t.Helper()

	c, err := e.app.Cache()
	if err != nil {
		t.Fatal(err)
	}

	svc, err := chat.New(chat.Config{DB: db(t, e.app), Cache: c, Now: now})
	if err != nil {
		t.Fatal(err)
	}

	return svc
}

// server serves the app over real HTTP (WebSocket tests).
func (e *env) server(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(e.app.Handler())
	t.Cleanup(srv.Close)

	return srv
}

// socket is a test client of the broadcast WebSocket protocol.
type socket struct {
	t    *testing.T
	conn *cws.Conn
}

// frame is a server message of the broadcast protocol.
type frame struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel"`
	Event   string          `json:"event"`
	Data    json.RawMessage `json:"data"`
	Status  int             `json:"status"`
	Message string          `json:"message"`
}

// dial opens /broadcast/ws on srv as userID (0: anonymous) from origin
// ("" sends the server's own origin). It returns the socket, or nil and
// the HTTP status of a refused upgrade.
func (e *env) dial(t *testing.T, srv *httptest.Server, userID int64, origin string) (*socket, int) {
	t.Helper()

	h := http.Header{}

	if origin == "" {
		origin = srv.URL
	}

	h.Set("Origin", origin)

	if userID != 0 {
		var parts []string
		for _, ck := range authtest.SessionCookies(t, e.app, userID) {
			parts = append(parts, ck.Name+"="+ck.Value)
		}

		h.Set("Cookie", strings.Join(parts, "; "))
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	conn, resp, err := cws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/broadcast/ws",
		&cws.DialOptions{HTTPHeader: h, HTTPClient: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		if resp == nil {
			t.Fatalf("dial: %v", err)
		}

		return nil, resp.StatusCode
	}

	t.Cleanup(func() { _ = conn.CloseNow() })

	return &socket{t: t, conn: conn}, http.StatusSwitchingProtocols
}

func (s *socket) subscribe(channel string) {
	s.t.Helper()

	b, err := json.Marshal(map[string]string{"type": "subscribe", "channel": channel})
	if err != nil {
		s.t.Fatal(err)
	}

	if err := s.conn.Write(s.t.Context(), cws.MessageText, b); err != nil {
		s.t.Fatalf("subscribe %s: %v", channel, err)
	}
}

// next reads frames until one matches want (type, and event when set) or
// the timeout passes.
func (s *socket) next(want, event string) frame {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Second)
	defer cancel()

	for {
		_, b, err := s.conn.Read(ctx)
		if err != nil {
			s.t.Fatalf("waiting for %s %s: %v", want, event, err)
		}

		var f frame
		if err := json.Unmarshal(b, &f); err != nil {
			s.t.Fatalf("bad frame %s: %v", b, err)
		}

		if f.Type == want && (event == "" || f.Event == event) {
			return f
		}
	}
}

// expectSubscribed subscribes and waits for the acknowledgement.
func (s *socket) expectSubscribed(channel string) {
	s.t.Helper()

	s.subscribe(channel)

	if f := s.next("subscribed", ""); f.Channel != channel {
		s.t.Fatalf("subscribed %q, want %q", f.Channel, channel)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// events reads event frames until one of each name arrived (frames of
// different channels may interleave in any order).
func (s *socket) events(names ...string) map[string]frame {
	s.t.Helper()

	out := map[string]frame{}

	for len(out) < len(names) {
		f := s.next("event", "")
		for _, n := range names {
			if f.Event == n {
				out[n] = f
			}
		}
	}

	return out
}
