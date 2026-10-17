// Package qna_test holds the HTTP-level tests of product questions:
// asking, the seller inbox and admin moderation (in-memory SQLite).
package qna_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/qna"
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

func newEnv(t *testing.T) *env {
	t.Helper()

	fake := queue.NewFake()
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{AppOptions: []vukem.Option{vukem.WithQueueDriver(fake)}})

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
	u := models.User{Name: name, Email: fmt.Sprintf("user%d@example.com", e.n), Password: "x", Role: models.RoleUser}
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
	e.exec(
		t,
		`UPDATE users SET role = ?, two_factor_confirmed_at = ? WHERE id = ?`,
		models.RoleAdmin,
		time.Now().UTC(),
		u.ID,
	)
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

	return shop, e.productOf(t, u, title, catalog.StatusPublished)
}

func (e *env) productOf(t *testing.T, u models.User, title, status string) catalog.SellerProduct {
	t.Helper()

	svc := e.catalog(t)

	p, err := svc.CreateProduct(t.Context(), u.ID, catalog.ProductInput{
		Title: title, Description: "About " + title, PriceCents: 19990, Stock: 3, WeightGrams: 500,
	})
	if err != nil {
		t.Fatal(err)
	}

	if status != catalog.StatusDraft {
		if p, err = svc.SetProductStatus(t.Context(), u.ID, p.ID, status); err != nil {
			t.Fatal(err)
		}
	}

	return p
}

// json returns a JSON client signed in as u (errors come back as status
// codes).
func (e *env) json(t *testing.T, u models.User) *vukemtest.Client {
	t.Helper()

	return authtest.ActingAs(t, e.app, e.app.Client(t).AcceptJSON(), u)
}

// inertia returns an Inertia client signed in as u.
func (e *env) inertia(t *testing.T, u models.User) *vukemtest.Client {
	t.Helper()

	return authtest.ActingAs(t, e.app, e.app.Client(t).Inertia(""), u)
}

func (e *env) askPath() string { return "/p/" + e.product.Slug + "/questions" }

// question inserts a question directly (no rate limit, no notification).
func (e *env) question(t *testing.T, asker models.User, body string, created time.Time) qna.Question {
	t.Helper()

	q := qna.Question{
		CreatedAt: created,
		ProductID: e.product.ID, ShopID: e.shop.ID, AskerID: asker.ID, Body: body,
	}

	if err := orm.Create(t.Context(), db(t, e.app), &q); err != nil {
		t.Fatal(err)
	}

	return q
}

func (e *env) load(t *testing.T, id int64) qna.Question {
	t.Helper()

	q, err := orm.QueryOf[qna.Question](db(t, e.app)).Find(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}

	return q
}

// mailJobs counts the queued mail jobs; their payload is sealed, so the
// recipient must not appear in clear text.
func (e *env) mailJobs(t *testing.T, email string) int {
	t.Helper()

	n := 0

	msgs := e.queue.Messages()
	for i := range msgs {
		if msgs[i].Kind != vukem.MailJobKind {
			continue
		}

		if strings.Contains(string(msgs[i].Payload), email) {
			t.Fatalf("queued mail payload shows %s in clear text", email)
		}

		n++
	}

	return n
}

// runQueued runs every recorded job, like a worker.
func (e *env) runQueued(t *testing.T) {
	t.Helper()

	q, err := e.app.Queue()
	if err != nil {
		t.Fatal(err)
	}

	for {
		m, err := e.queue.Pop(t.Context(), []string{"default"})
		if err != nil {
			t.Fatal(err)
		}

		if m == nil {
			return
		}

		if err := q.Process(t.Context(), m); err != nil {
			t.Fatalf("job %s: %v", m.Kind, err)
		}
	}
}

func id(v int64) string { return strconv.FormatInt(v, 10) }
