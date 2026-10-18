package catalog_test

import (
	"encoding/binary"
	"hash/crc32"
	"image"
	"net/http"
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/catalog"
)

func images(t *testing.T, app *vukemtest.App, productID int64) []catalog.ProductImage {
	t.Helper()

	rows, err := orm.QueryOf[catalog.ProductImage](mustDB(t, app)).Where("product_id", productID).
		OrderBy("position", orm.Asc).Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return rows
}

func TestUploadImages(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, _ := seller(t, app, "img@example.com", "Loja Img", "SP")
	p := product(t, app, u, productSpec{title: "Produto Com Foto", price: 1000, stock: 1})
	c := asJSON(t, app, u)
	url := "/seller/products/" + id(p.ID) + "/images"

	body, ctype := multipartBody(t,
		filePart{"wide.jpg", "image/jpeg", jpegBytes(t, 2000, 1000)},
		filePart{"small.png", "image/png", pngBytes(t, 200, 100)},
	)
	c.Do(http.MethodPost, url, body, ctype).AssertRedirect("/seller/products/" + id(p.ID) + "/edit")

	imgs := images(t, app, p.ID)
	// Width/height are the largest variant's: 1280 wide for the 2000 px
	// photo, the original 200 px for the small one (never upscaled).
	if len(imgs) != 2 || imgs[0].Width != 1280 || imgs[0].Height != 640 || imgs[1].Width != 200 ||
		imgs[1].Position != 1 {
		t.Fatalf("images = %+v", imgs)
	}

	guest := app.Client(t)

	for _, tt := range []struct {
		img   catalog.ProductImage
		width int
		want  int
	}{
		{imgs[0], 1280, 1280},
		{imgs[0], 640, 640},
		{imgs[0], 320, 320},
		{imgs[1], 1280, 200},
		{imgs[1], 320, 200}, // never upscaled
	} {
		res := guest.Get(catalog.MediaURL(tt.img.Key, tt.width)).AssertOK().
			AssertHeader("Content-Type", "image/jpeg").AssertHeader("X-Content-Type-Options", "nosniff")

		cfg, format, err := image.DecodeConfig(strings.NewReader(res.Body()))
		if err != nil || format != "jpeg" || cfg.Width != tt.want {
			t.Fatalf(
				"variant w%d of %s: %v %s %dx%d, want jpeg %d wide",
				tt.width,
				tt.img.Key,
				err,
				format,
				cfg.Width,
				cfg.Height,
				tt.want,
			)
		}
	}

	// The public product page lists the images in order.
	app.Client(t).Inertia("").Get("/p/"+p.Slug).AssertOK().AssertInertia("Catalog/Product").
		HasProp("product.images", 2).PropEquals("product.images.0.url_640", catalog.MediaURL(imgs[0].Key, 640))

	// Reorder, then delete.
	c.PutJSON(url+"/order", map[string]any{"ids": []int64{imgs[1].ID, imgs[0].ID}}).AssertStatus(http.StatusSeeOther)

	if got := images(t, app, p.ID); got[0].ID != imgs[1].ID {
		t.Fatalf("after reorder first image = %d, want %d", got[0].ID, imgs[1].ID)
	}

	c.PutJSON(url+"/order", map[string]any{"ids": []int64{imgs[1].ID}}).AssertUnprocessable()        // incomplete
	c.PutJSON(url+"/order", map[string]any{"ids": []int64{imgs[1].ID, 99999}}).AssertUnprocessable() // foreign id

	c.Delete(url + "/" + id(imgs[0].ID)).AssertStatus(http.StatusSeeOther)
	guest.Get(catalog.MediaURL(imgs[0].Key, 640)).AssertNotFound()

	if got := images(t, app, p.ID); len(got) != 1 {
		t.Fatalf("after delete %d images, want 1", len(got))
	}
}

func TestUploadMaxImages(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, _ := seller(t, app, "max@example.com", "Loja Max", "SP")
	p := product(t, app, u, productSpec{title: "Muitas Fotos", price: 1000, stock: 1})
	c := asJSON(t, app, u)
	url := "/seller/products/" + id(p.ID) + "/images"

	nine := make([]filePart, 9)
	for i := range nine {
		nine[i] = filePart{"p.jpg", "image/jpeg", jpegBytes(t, 20, 20)}
	}

	body, ctype := multipartBody(t, nine...)
	c.Do(http.MethodPost, url, body, ctype).AssertUnprocessable()
	vukemtest.AssertDatabaseCount(t, app, "product_images", 0)

	body, ctype = multipartBody(t, nine[:8]...)
	c.Do(http.MethodPost, url, body, ctype).AssertStatus(http.StatusSeeOther)

	body, ctype = multipartBody(t, nine[:1]...)
	c.Do(http.MethodPost, url, body, ctype).AssertUnprocessable()
	vukemtest.AssertDatabaseCount(t, app, "product_images", 8)
}

// pngWithSize returns a tiny valid PNG whose header claims w×h pixels: a
// decompression bomb as far as a naive decoder is concerned.
func pngWithSize(t *testing.T, w, h uint32) []byte {
	t.Helper()

	b := pngBytes(t, 1, 1)
	// IHDR data starts after the 8-byte signature, 4-byte length and type.
	binary.BigEndian.PutUint32(b[16:], w)
	binary.BigEndian.PutUint32(b[20:], h)
	binary.BigEndian.PutUint32(b[29:], crc32.ChecksumIEEE(b[12:29]))

	return b
}

// TestUploadAbuse: every hostile file is rejected before anything is
// stored, and accepted files are re-encoded (no payload survives).
func TestUploadAbuse(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, _ := seller(t, app, "abuse@example.com", "Loja Abuse", "SP")
	p := product(t, app, u, productSpec{title: "Alvo", price: 1000, stock: 1})
	c := asJSON(t, app, u)
	url := "/seller/products/" + id(p.ID) + "/images"

	html := []byte(`<html><body><script>alert(document.cookie)</script></body></html>`)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>alert(1)</script></svg>`)
	big := append(jpegBytes(t, 10, 10), make([]byte, catalog.MaxImageBytes)...)

	tests := []struct {
		name   string
		file   filePart
		status int
	}{
		{"html first polyglot", filePart{"x.jpg", "image/jpeg", append(html, jpegBytes(t, 10, 10)...)}, 422},
		{"svg with script", filePart{"x.svg", "image/svg+xml", svg}, 422},
		{"svg renamed png", filePart{"x.png", "image/png", svg}, 422},
		{"decompression bomb", filePart{"bomb.png", "image/png", pngWithSize(t, 50_000, 50_000)}, 422},
		{"just over the pixel cap", filePart{"cap.png", "image/png", pngWithSize(t, 4001, 4000)}, 422},
		{"oversize", filePart{"big.jpg", "image/jpeg", big}, 413},
		{"wrong extension", filePart{"photo.jpg", "image/jpeg", pngBytes(t, 10, 10)}, 422},
		{"executable extension", filePart{"photo.php", "image/jpeg", jpegBytes(t, 10, 10)}, 422},
		// A GIF header without image data passes upload; the job drops it.
		{"gif without pixels", filePart{"x.gif", "image/gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")}, 303},
	}

	for _, tt := range tests {
		body, ctype := multipartBody(t, tt.file)
		c.Do(http.MethodPost, url, body, ctype).AssertStatus(tt.status)
	}

	vukemtest.AssertDatabaseCount(t, app, "product_images", 0)

	// A truncated JPEG passes the header checks; the thumbnail job cannot
	// decode it and drops the image.
	body, ctype := multipartBody(t, filePart{"t.jpg", "image/jpeg", jpegBytes(t, 400, 400)[:600]})
	c.Do(http.MethodPost, url, body, ctype).AssertStatus(http.StatusSeeOther)
	vukemtest.AssertDatabaseCount(t, app, "product_images", 0)

	// JPEG with an HTML payload appended: a valid JPEG, accepted, but the
	// stored variants are re-encoded, so the payload is gone.
	body, ctype = multipartBody(t, filePart{"poly.jpg", "image/jpeg", append(jpegBytes(t, 64, 64), html...)})
	c.Do(http.MethodPost, url, body, ctype).AssertStatus(http.StatusSeeOther)

	imgs := images(t, app, p.ID)
	if len(imgs) != 1 {
		t.Fatalf("polyglot upload stored %d images, want 1", len(imgs))
	}

	for _, w := range catalog.Widths {
		got := app.Client(t).Get(catalog.MediaURL(imgs[0].Key, w)).AssertOK().Body()
		if strings.Contains(got, "<script") || strings.Contains(got, "<html") {
			t.Fatalf("variant w%d still carries the HTML payload", w)
		}
	}
}

// TestMediaRouteRejectsArbitraryKeys: only well-formed image keys reach
// the disk.
func TestMediaRouteRejectsArbitraryKeys(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	guest := app.Client(t)
	name := strings.Repeat("a2", 13) // the shape of upload.Receive names

	for _, path := range []string{
		"/media/products/1/" + name + "-w640.jpg", // well formed, missing
		"/media/products/1/" + name + "-w641.jpg",
		"/media/products/1/" + name + ".svg",
		"/media/products/1/" + name + ".html",
		"/media/users/1/" + name + ".jpg",
		"/media/products/x/" + name + ".jpg",
		"/media/products/1/" + strings.ToUpper(name) + ".jpg",
		"/media/products/1/..%2f..%2f.env",
		"/media/products/1/%2e" + name + ".jpg",
		"/media/products/1/" + name + ".jpg%2f..%2f..%2fx.jpg",
	} {
		guest.Get(path).AssertNotFound()
	}
}
