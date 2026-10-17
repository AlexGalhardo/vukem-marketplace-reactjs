// Package apifixture seeds users, API keys, a small catalog and orders
// for the API tests (tests/api, tests/graphql).
package apifixture

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/sqlbind"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/authtest"
)

// NewApp boots the app on in-memory SQLite with extra env.
func NewApp(tb testing.TB, env map[string]string) *vukemtest.App {
	tb.Helper()

	return vukemtest.New(tb, bootstrap.New, vukemtest.Options{Env: env})
}

// ActingAs logs c in as u with a stamped session.
func ActingAs(tb testing.TB, app *vukemtest.App, c *vukemtest.Client, u models.User) *vukemtest.Client {
	tb.Helper()

	return authtest.ActingAs(tb, app, c, u)
}

// DB returns the app database.
func DB(tb testing.TB, app *vukemtest.App) *database.DB {
	tb.Helper()

	d, err := app.DB()
	if err != nil {
		tb.Fatal(err)
	}

	return d
}

// Exec runs a statement written with ? placeholders.
func Exec(tb testing.TB, app *vukemtest.App, query string, args ...any) {
	tb.Helper()

	d := DB(tb, app)
	if _, err := d.ExecContext(context.WithoutCancel(tb.Context()), sqlbind.Rebind(d.Dialect(), query),
		args...); err != nil {
		tb.Fatalf("%s: %v", query, err)
	}
}

func insertID(tb testing.TB, app *vukemtest.App, query string, args ...any) int64 {
	tb.Helper()

	d := DB(tb, app)

	var id int64
	if err := d.QueryRowContext(context.WithoutCancel(tb.Context()), sqlbind.Rebind(d.Dialect(), query+" RETURNING id"),
		args...).Scan(&id); err != nil {
		tb.Fatalf("%s: %v", query, err)
	}

	return id
}

// MakeUser inserts a user.
func MakeUser(tb testing.TB, app *vukemtest.App, email string) models.User {
	tb.Helper()

	u := models.User{Name: "User " + email, Email: email, Password: "unused", Role: models.RoleUser}
	if err := orm.Create(context.WithoutCancel(tb.Context()), DB(tb, app), &u); err != nil {
		tb.Fatal(err)
	}

	return u
}

// Keys returns the API key store over the app database.
func Keys(tb testing.TB, app *vukemtest.App) *api.Keys {
	tb.Helper()

	return api.NewKeys(DB(tb, app), time.Now)
}

// Token creates an API key for u and returns its plain token.
func Token(tb testing.TB, app *vukemtest.App, u models.User, scopes ...string) string {
	tb.Helper()

	_, tok, err := Keys(
		tb,
		app,
	).Create(context.WithoutCancel(tb.Context()), u.ID, api.NewKey{Name: "test", Scopes: scopes})
	if err != nil {
		tb.Fatal(err)
	}

	return tok
}

// Fixture is a small catalog: Ana's active shop with a published, a draft
// and a cheap product, plus Bia's suspended shop with a published product.
type Fixture struct {
	Ana, Bia, Carl models.User
	AnaShop        int64
	Published      string // ana's, price 1990, stock 5
	Cheap          string // ana's, price 500, stock 100
	Draft          string // ana's, draft
	Hidden         string // bia's suspended shop
}

// SeedCatalog inserts the Fixture catalog.
func SeedCatalog(tb testing.TB, app *vukemtest.App) Fixture {
	tb.Helper()

	f := Fixture{
		Ana: MakeUser(tb, app, "ana@example.com"), Bia: MakeUser(tb, app, "bia@example.com"),
		Carl:      MakeUser(tb, app, "carl@example.com"),
		Published: "lamp", Cheap: "pen", Draft: "secret-draft", Hidden: "suspended-mug",
	}
	now := time.Now().UTC()

	cat := insertID(tb, app, `INSERT INTO categories (slug, name_en, name_pt, position, created_at, updated_at)
		VALUES ('home', 'Home', 'Casa', 0, ?, ?)`, now, now)
	Exec(tb, app, `INSERT INTO categories (slug, name_en, name_pt, parent_id, position, created_at, updated_at)
		VALUES ('lighting', 'Lighting', 'Iluminação', ?, 1, ?, ?)`, cat, now, now)

	f.AnaShop = insertID(tb, app, `INSERT INTO shops (owner_id, slug, name, description, status, stripe_ready,
		ship_from_uf, created_at, updated_at) VALUES (?, 'ana-store', 'Ana Store', 'Lamps and pens', 'active', ?, 'SP', ?, ?)`,
		f.Ana.ID, true, now, now)
	biaShop := insertID(tb, app, `INSERT INTO shops (owner_id, slug, name, status, stripe_ready, ship_from_uf,
		created_at, updated_at) VALUES (?, 'bia-store', 'Bia Store', 'suspended', ?, 'RJ', ?, ?)`, f.Bia.ID, false, now, now)

	product := func(shop int64, slug, title, status string, price int64, stock int) int64 {
		return insertID(tb, app, `INSERT INTO products (shop_id, category_id, slug, title, description, price_cents,
			stock, weight_grams, status, rating_avg_x100, rating_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'A fine thing', ?, ?, 300, ?, 0, 0, ?, ?)`,
			shop, cat, slug, title, price, stock, status, now, now)
	}

	lamp := product(f.AnaShop, f.Published, "Desk Lamp", "published", 1990, 5)
	product(f.AnaShop, f.Cheap, "Blue Pen", "published", 500, 100)
	product(f.AnaShop, f.Draft, "Secret Draft", "draft", 100, 1)
	product(biaShop, f.Hidden, "Suspended Mug", "published", 900, 3)

	Exec(tb, app, `INSERT INTO product_images (product_id, key, width, height, position, alt, created_at, updated_at)
		VALUES (?, 'products/lamp/1', 1280, 960, 0, 'A desk lamp', ?, ?)`, lamp, now, now)

	return f
}

// SeedOrder creates a paid order of buyer with one line of ana's lamp.
func SeedOrder(tb testing.TB, app *vukemtest.App, f Fixture, buyer models.User) string {
	tb.Helper()

	now := time.Now().UTC()
	public := uuid.Must(uuid.NewV7()).String()

	order := insertID(tb, app, `INSERT INTO orders (public_id, buyer_id, status, payment_method, subtotal_cents,
		shipping_cents, fee_cents, total_cents, shipping_address, created_at, updated_at)
		VALUES (?, ?, 'paid', 'pix', 1990, 1500, 199, 3490, '{}', ?, ?)`, public, buyer.ID, now, now)
	so := insertID(tb, app, `INSERT INTO seller_orders (order_id, shop_id, status, subtotal_cents, shipping_cents,
		fee_cents, payout_cents, created_at, updated_at) VALUES (?, ?, 'paid', 1990, 1500, 199, 3291, ?, ?)`,
		order, f.AnaShop, now, now)
	Exec(tb, app, `INSERT INTO order_items (seller_order_id, product_id, title, price_cents, quantity, weight_grams,
		created_at, updated_at) VALUES (?, (SELECT id FROM products WHERE slug = ?), 'Desk Lamp', 1990, 1, 300, ?, ?)`,
		so, f.Published, now, now)

	return public
}

// Bearer is the Authorization header value for tok.
func Bearer(tok string) string { return "Bearer " + tok }

// MustFind returns one column of one row as a string ("" for NULL).
func MustFind(tb testing.TB, app *vukemtest.App, query string, args ...any) string {
	tb.Helper()

	d := DB(tb, app)

	var v sql.NullString
	if err := d.QueryRowContext(context.WithoutCancel(tb.Context()), sqlbind.Rebind(d.Dialect(), query), args...).
		Scan(&v); err != nil {
		tb.Fatalf("%s: %v", query, err)
	}

	return v.String
}
