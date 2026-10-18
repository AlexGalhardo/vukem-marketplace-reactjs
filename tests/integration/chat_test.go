//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	cws "github.com/coder/websocket"

	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/chat"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/authtest"
)

// chatInstance is one app instance served over real HTTP; every instance
// of a test shares Postgres, Redis (pub/sub, cache, sessions) and the
// Redis prefix.
type chatInstance struct {
	app *vukemtest.App
	srv *httptest.Server
}

func chatInstances(t *testing.T, n int) []chatInstance {
	t.Helper()

	dbURL := env(t, "TEST_DATABASE_URL")
	// Redis is resolved before vukem.New reads the app source (see
	// platform.RedisOptions), so it comes from the process environment. A
	// fresh prefix keeps this run's channels and keys apart.
	t.Setenv("REDIS_URL", env(t, "TEST_REDIS_URL"))
	t.Setenv("REDIS_PREFIX", "it-chat-"+strings.ToLower(rand.Text()[:8])+":")

	out := make([]chatInstance, n)
	for i := range out {
		app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
			"DATABASE_URL":     dbURL,
			"RATE_LIMIT_STORE": "cache",
		}})

		srv := httptest.NewServer(app.Handler())
		t.Cleanup(srv.Close)

		out[i] = chatInstance{app: app, srv: srv}
	}

	return out
}

// verifiedUser creates a verified user (the email test may switch
// email_verification on in a shared database).
func verifiedUser(t *testing.T, app *vukemtest.App, name string) models.User {
	t.Helper()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	u := models.User{Name: name, Email: word(12) + "@example.com", Password: "x", Role: models.RoleUser}
	u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = time.Now().UTC(), true

	if err := orm.Create(t.Context(), db, &u); err != nil {
		t.Fatal(err)
	}

	return u
}

// chatProduct opens a shop for a new seller with one published product.
func chatProduct(t *testing.T, app *vukemtest.App) (models.User, catalog.SellerProduct) {
	t.Helper()

	svc, err := catalog.FromApp(app.App, nil)
	if err != nil {
		t.Fatal(err)
	}

	owner := verifiedUser(t, app, "Olga Owner")

	if _, err := svc.OpenShop(
		t.Context(),
		owner.ID,
		catalog.ShopInput{Name: "Loja " + word(6), ShipFromUF: "SP"},
	); err != nil {
		t.Fatal(err)
	}

	p, err := svc.CreateProduct(t.Context(), owner.ID, catalog.ProductInput{
		Title: "Chaleira " + word(8), Description: "x", PriceCents: 15000, Stock: 2, WeightGrams: 800,
	})
	if err != nil {
		t.Fatal(err)
	}

	if p, err = svc.SetProductStatus(t.Context(), owner.ID, p.ID, catalog.StatusPublished); err != nil {
		t.Fatal(err)
	}

	return owner, p
}

type wsFrame struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel"`
	Event   string          `json:"event"`
	Data    json.RawMessage `json:"data"`
	Status  int             `json:"status"`
}

type wsClient struct {
	t    *testing.T
	conn *cws.Conn
}

func dialChat(t *testing.T, in chatInstance, userID int64) *wsClient {
	t.Helper()

	var parts []string
	for _, ck := range authtest.SessionCookies(t, in.app, userID) {
		parts = append(parts, ck.Name+"="+ck.Value)
	}

	h := http.Header{"Origin": {in.srv.URL}, "Cookie": {strings.Join(parts, "; ")}}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	conn, resp, err := cws.Dial(ctx, "ws"+strings.TrimPrefix(in.srv.URL, "http")+"/broadcast/ws",
		&cws.DialOptions{HTTPHeader: h})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	t.Cleanup(func() { _ = conn.CloseNow() })

	return &wsClient{t: t, conn: conn}
}

func (c *wsClient) send(v any) {
	c.t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		c.t.Fatal(err)
	}

	if err := c.conn.Write(c.t.Context(), cws.MessageText, b); err != nil {
		c.t.Fatal(err)
	}
}

func (c *wsClient) next(want, event string) (wsFrame, bool) {
	c.t.Helper()

	ctx, cancel := context.WithTimeout(c.t.Context(), 10*time.Second)
	defer cancel()

	for {
		_, b, err := c.conn.Read(ctx)
		if err != nil {
			return wsFrame{}, false
		}

		var f wsFrame
		if err := json.Unmarshal(b, &f); err != nil {
			c.t.Fatalf("bad frame %s", b)
		}

		if f.Type == want && (event == "" || f.Event == event) {
			return f, true
		}
	}
}

func (c *wsClient) subscribe(channel string) {
	c.t.Helper()

	c.send(map[string]string{"type": "subscribe", "channel": channel})

	if f, ok := c.next("subscribed", ""); !ok || f.Channel != channel {
		c.t.Fatalf("subscribe %s: %+v ok=%v", channel, f, ok)
	}
}

func idOf(t *testing.T, res *vukemtest.Response) int64 {
	t.Helper()

	loc := res.Header("Location")

	id, err := strconv.ParseInt(strings.TrimPrefix(loc, "/messages/"), 10, 64)
	if err != nil {
		t.Fatalf("start: status %d location %q body %s", res.Status(), loc, res.Body())
	}

	return id
}

// TestChatAcrossInstances runs two app instances on one Postgres and one
// Redis: a buyer connected to instance A and a seller connected to
// instance B talk live, the buyer's message posted on A reaching B
// through Redis pub/sub, and B's reply reaching A.
func TestChatAcrossInstances(t *testing.T) {
	in := chatInstances(t, 2)
	a, b := in[0], in[1]

	owner, p := chatProduct(t, a.app)
	buyer := verifiedUser(t, a.app, "Bia Buyer")

	id := idOf(t, authtest.ActingAs(t, a.app, a.app.Client(t).AcceptJSON(), buyer).
		PostJSON("/messages", map[string]any{"product": p.Slug}))

	onA := dialChat(t, a, buyer.ID)
	onB := dialChat(t, b, owner.ID)

	onA.subscribe(chat.MessagesChannel(id))
	onB.subscribe(chat.MessagesChannel(id))
	onB.subscribe(chat.InboxChannel(owner.ID))

	authtest.ActingAs(t, a.app, a.app.Client(t).AcceptJSON(), buyer).
		PostJSON("/messages/"+strconv.FormatInt(id, 10), map[string]any{"body": "Oi, vale a pena?"}).
		AssertCreated()

	f, ok := onB.next("event", chat.EventMessage)
	if !ok || !strings.Contains(string(f.Data), "Oi, vale a pena?") {
		t.Fatalf("seller on B: %+v ok=%v", f, ok)
	}

	if f, ok := onB.next("event", chat.EventInbox); !ok || !strings.Contains(string(f.Data), `"unread":1`) {
		t.Fatalf("seller inbox on B: %+v ok=%v", f, ok)
	}

	authtest.ActingAs(t, b.app, b.app.Client(t).AcceptJSON(), owner).
		PostJSON("/messages/"+strconv.FormatInt(id, 10), map[string]any{"body": "Vale sim!"}).AssertCreated()

	// The buyer's own message arrives first, then the reply from B.
	for got := ""; !strings.Contains(got, "Vale sim!"); {
		f, ok := onA.next("event", chat.EventMessage)
		if !ok {
			t.Fatal("buyer on A never received the reply posted on B")
		}

		got = string(f.Data)
	}

	// An outsider on B cannot subscribe to the conversation.
	mallory := verifiedUser(t, b.app, "Mallory")
	onM := dialChat(t, b, mallory.ID)
	onM.send(map[string]string{"type": "subscribe", "channel": chat.MessagesChannel(id)})

	if f, ok := onM.next("error", ""); !ok || f.Status != http.StatusForbidden {
		t.Fatalf("outsider: %+v ok=%v, want 403", f, ok)
	}

	// Last-seen lives in the shared cache: B's page knows the buyer, whose
	// socket is on A, is online.
	authtest.ActingAs(t, b.app, b.app.Client(t).Inertia(""), owner).Get("/messages/"+strconv.FormatInt(id, 10)).
		AssertOK().AssertInertia("Messages/Show").PropEquals("online", true)
}

// TestChatSQLOnPostgres runs the inbox, unread and catch-up SQL and the
// thread uniqueness index on Postgres.
func TestChatSQLOnPostgres(t *testing.T) {
	in := chatInstances(t, 1)
	app := in[0].app

	owner, p := chatProduct(t, app)
	buyer := verifiedUser(t, app, "Caio Comprador")
	bc := authtest.ActingAs(t, app, app.Client(t).AcceptJSON(), buyer)

	id := idOf(t, bc.PostJSON("/messages", map[string]any{"product": p.Slug}))
	if again := idOf(t, bc.PostJSON("/messages", map[string]any{"product": p.Slug})); again != id {
		t.Fatalf("second start %d, want %d", again, id)
	}

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	// The expression index keeps one thread per buyer, shop and product,
	// NULL product included.
	_, err = db.ExecContext(t.Context(),
		`INSERT INTO conversations (buyer_id, shop_id, product_id, created_at, updated_at)
		SELECT buyer_id, shop_id, product_id, NOW(), NOW() FROM conversations WHERE id = $1`, id)
	if err == nil {
		t.Fatal("duplicate conversation accepted")
	}

	shopThread := idOf(t, bc.PostJSON("/messages", map[string]any{"shop": shopSlug(t, app, owner.ID)}))

	_, err = db.ExecContext(t.Context(),
		`INSERT INTO conversations (buyer_id, shop_id, created_at, updated_at)
		SELECT buyer_id, shop_id, NOW(), NOW() FROM conversations WHERE id = $1`, shopThread)
	if err == nil {
		t.Fatal("duplicate product-less conversation accepted")
	}

	for i := range 3 {
		bc.PostJSON("/messages/"+strconv.FormatInt(id, 10), map[string]any{"body": fmt.Sprintf("mensagem %d", i)}).
			AssertCreated()
	}

	sc := authtest.ActingAs(t, app, app.Client(t).Inertia(""), owner)
	sc.Get("/seller/messages").AssertOK().AssertInertia("Messages/Index").
		PropEquals("chat.unread", 3).PropEquals("conversations.data.0.id", id).
		PropEquals("conversations.data.0.unread", 3).PropEquals("conversations.data.0.last_message", "mensagem 2")

	bc.Get("/messages/"+strconv.FormatInt(id, 10)+"/since?after=0").AssertOK().AssertJSONCount("messages", 3)
	sc.Get("/messages/"+strconv.FormatInt(id, 10)).AssertOK().AssertInertia("Messages/Show").HasProp("messages", 3)
	sc.Get("/seller/messages").AssertOK().AssertInertia("Messages/Index").PropEquals("chat.unread", 0)
}

func shopSlug(t *testing.T, app *vukemtest.App, ownerID int64) string {
	t.Helper()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	shop, err := orm.QueryOf[catalog.Shop](db).Where("owner_id", ownerID).First(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return shop.Slug
}
