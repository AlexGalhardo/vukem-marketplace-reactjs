// Package catalog_test holds the HTTP-level tests of the catalog: shops,
// products, image uploads and search (in-memory SQLite; the Postgres full
// text search runs in tests/integration).
package catalog_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/textproto"
	"strconv"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
)

func newApp(t *testing.T) *vukemtest.App {
	t.Helper()

	app := vukemtest.New(t, bootstrap.New)
	if err := (seeders.Categories{}).Seed(t.Context(), mustDB(t, app)); err != nil {
		t.Fatal(err)
	}

	return app
}

func mustDB(t *testing.T, app *vukemtest.App) *database.DB {
	t.Helper()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	return db
}

// newUser creates a user directly (registration is covered by the auth tests).
func newUser(t *testing.T, app *vukemtest.App, email string, verified bool) models.User {
	t.Helper()

	u := models.User{Name: "Seller " + email, Email: email, Password: "x", Role: models.RoleUser}
	if verified {
		u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = time.Now().UTC(), true
	}

	if err := orm.Create(t.Context(), mustDB(t, app), &u); err != nil {
		t.Fatal(err)
	}

	return u
}

// as returns an Inertia client signed in as u.
func as(t *testing.T, app *vukemtest.App, u models.User) *vukemtest.Client {
	t.Helper()

	return app.Client(t).Inertia("").ActingAs("web", u)
}

// asJSON returns a JSON client signed in as u: errors come back as status
// codes instead of Inertia redirects.
func asJSON(t *testing.T, app *vukemtest.App, u models.User) *vukemtest.Client {
	t.Helper()

	return app.Client(t).AcceptJSON().ActingAs("web", u)
}

func service(t *testing.T, app *vukemtest.App) *catalog.Service {
	t.Helper()

	svc, err := catalog.FromApp(app.App, nil)
	if err != nil {
		t.Fatal(err)
	}

	return svc
}

// seller creates a verified user with a shop.
func seller(t *testing.T, app *vukemtest.App, email, shopName, uf string) (models.User, catalog.ShopView) {
	t.Helper()

	u := newUser(t, app, email, true)

	shop, err := service(t, app).OpenShop(t.Context(), u.ID, catalog.ShopInput{Name: shopName, ShipFromUF: uf})
	if err != nil {
		t.Fatal(err)
	}

	return u, shop
}

type productSpec struct {
	title, category, status string
	price                   int64
	stock                   int
}

// product creates a product through the service (status defaults to published).
func product(t *testing.T, app *vukemtest.App, owner models.User, p productSpec) catalog.SellerProduct {
	t.Helper()

	svc := service(t, app)

	sp, err := svc.CreateProduct(t.Context(), owner.ID, catalog.ProductInput{
		Title: p.title, Description: "Description of " + p.title, CategorySlug: p.category,
		PriceCents: p.price, Stock: p.stock, WeightGrams: 300,
	})
	if err != nil {
		t.Fatal(err)
	}

	status := p.status
	if status == "" {
		status = catalog.StatusPublished
	}

	if status != catalog.StatusDraft {
		if sp, err = svc.SetProductStatus(t.Context(), owner.ID, sp.ID, status); err != nil {
			t.Fatal(err)
		}
	}

	return sp
}

func id(v int64) string { return strconv.FormatInt(v, 10) }

// filePart is one file in a multipart body.
type filePart struct {
	name, contentType string
	data              []byte
}

// multipartBody builds a multipart/form-data body with files in field "images".
func multipartBody(t *testing.T, files ...filePart) (*bytes.Buffer, string) {
	t.Helper()

	var buf bytes.Buffer

	w := multipart.NewWriter(&buf)

	for _, f := range files {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="images"; filename=%q`, f.name))
		h.Set("Content-Type", f.contentType)

		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := part.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	return &buf, w.FormDataContentType()
}

func testImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}

	return img
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testImage(w, h), nil); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage(w, h)); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}
