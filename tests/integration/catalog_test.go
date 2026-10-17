//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"maps"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/mongo"
	"github.com/AlexGalhardo/vukem/drivers/s3"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/storage"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/authtest"
)

// word returns n random lower-case letters: a search token no other test
// run shares (the database is not reset between runs).
func word(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)

	for i := range b {
		b[i] = 'a' + b[i]%26
	}

	return string(b)
}

// catalogSeller creates a verified user with a shop through the service.
func catalogSeller(t *testing.T, svc *catalog.Service, app *vukemtest.App, uf string) models.User {
	t.Helper()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	u := models.User{Name: "Seller", Email: word(12) + "@example.com", Password: "x", Role: models.RoleUser}
	u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = time.Now().UTC(), true

	if err := orm.Create(t.Context(), db, &u); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.OpenShop(
		t.Context(),
		u.ID,
		catalog.ShopInput{Name: "Loja " + word(6), ShipFromUF: uf},
	); err != nil {
		t.Fatal(err)
	}

	return u
}

func catalogApp(t *testing.T, extra map[string]string) (*vukemtest.App, *catalog.Service) {
	t.Helper()

	envs := map[string]string{"DATABASE_URL": env(t, "TEST_DATABASE_URL")}
	maps.Copy(envs, extra)

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: envs})

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	if err := (seeders.Categories{}).Seed(t.Context(), db); err != nil {
		t.Fatal(err)
	}

	svc, err := catalog.FromApp(app.App, nil)
	if err != nil {
		t.Fatal(err)
	}

	return app, svc
}

func searchTitles(t *testing.T, c *vukemtest.Client, query string) []string {
	t.Helper()

	res := c.Get("/search?" + query).AssertOK().AssertInertia("Catalog/Search")
	raw, _ := res.Prop("results.data")
	items, _ := raw.([]any)

	out := []string{}

	for _, it := range items {
		m, _ := it.(map[string]any)
		s, _ := m["title"].(string)
		out = append(out, s)
	}

	return out
}

// TestSearchPostgres runs the real search: websearch_to_tsquery over the
// generated tsvector, the trigram fallback for typos, ranking, filters,
// and the Mongo search history when TEST_MONGO_URL is set.
func TestSearchPostgres(t *testing.T) {
	mongoURL := os.Getenv("TEST_MONGO_URL")

	extra := map[string]string{}
	if mongoURL != "" {
		extra["MONGO_URL"] = mongoURL
	}

	app, svc := catalogApp(t, extra)
	tok := word(14)
	sp := catalogSeller(t, svc, app, "SP")
	rj := catalogSeller(t, svc, app, "RJ")

	for _, p := range []struct {
		owner      models.User
		title, cat string
		desc       string
		price      int64
		status     string
	}{
		{sp, "Fone Bluetooth " + tok, "electronics", "", 15000, catalog.StatusPublished},
		{sp, "Fone com Fio " + tok, "electronics", "", 5000, catalog.StatusPublished},
		{rj, "Fone Gamer " + tok, "electronics", "", 30000, catalog.StatusPublished},
		{rj, "Camiseta Básica", "fashion", "Combina com o fone " + tok, 8000, catalog.StatusPublished},
		{sp, "Fone Rascunho " + tok, "electronics", "", 100, catalog.StatusDraft},
	} {
		created, err := svc.CreateProduct(t.Context(), p.owner.ID, catalog.ProductInput{
			Title: p.title, Description: p.desc, CategorySlug: p.cat, PriceCents: p.price, Stock: 2, WeightGrams: 100,
		})
		if err != nil {
			t.Fatal(err)
		}

		if p.status == catalog.StatusPublished {
			if _, err := svc.SetProductStatus(t.Context(), p.owner.ID, created.ID, p.status); err != nil {
				t.Fatal(err)
			}
		}
	}

	guest := app.Client(t).Inertia("")
	typo := tok[:6] + string('a'+(tok[6]-'a'+1)%26) + tok[7:]

	tests := []struct {
		query string
		want  []string
	}{
		// Title matches (weight A) rank above the description-only match (B).
		{"q=" + tok + "&sort=relevance", nil},
		{"q=" + url.QueryEscape("fone "+tok) + "&sort=price_asc", []string{
			"Fone com Fio " + tok, "Camiseta Básica", "Fone Bluetooth " + tok, "Fone Gamer " + tok,
		}},
		{"q=" + url.QueryEscape(tok+" -camiseta") + "&sort=price_desc", []string{
			"Fone Gamer " + tok, "Fone Bluetooth " + tok, "Fone com Fio " + tok,
		}},
		{"q=" + url.QueryEscape(`"fone gamer" `+tok), []string{"Fone Gamer " + tok}},
		{"q=" + tok + "&uf=RJ&sort=price_asc", []string{"Camiseta Básica", "Fone Gamer " + tok}},
		{"q=" + tok + "&category=fashion", []string{"Camiseta Básica"}},
		{"q=" + tok + "&min=6000&max=20000&sort=price_asc", []string{"Camiseta Básica", "Fone Bluetooth " + tok}},
		{
			"q=" + typo + "&sort=price_asc",
			[]string{"Fone com Fio " + tok, "Fone Bluetooth " + tok, "Fone Gamer " + tok},
		},
		{"q=" + url.QueryEscape(tok+"'); DROP TABLE products; --"), []string{}},
		{"q=" + url.QueryEscape(`!!! & | :* <-> `+tok+` (`), nil},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := searchTitles(t, guest, tt.query)
			if tt.want == nil {
				return // only "no error"
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("results (-want +got):\n%s", diff)
			}
		})
	}

	rel := searchTitles(t, guest, "q="+tok+"&sort=relevance&per_page=48")
	if len(rel) != 4 || rel[3] != "Camiseta Básica" {
		t.Fatalf("relevance order = %q, want the description-only match last", rel)
	}

	if mongoURL == "" {
		t.Log("TEST_MONGO_URL not set: search history not checked")

		return
	}

	res := guest.Get("/search?q=" + url.QueryEscape("fone gamer "+tok)).AssertOK().AssertInertia("Catalog/Search")
	pid, _ := res.Prop("results.data.0.id")
	slug, _ := res.Prop("results.data.0.slug")
	guest.Get("/p/" + fmt.Sprint(slug)).AssertOK()

	checkSearchHistory(t, mongoURL, tok, toInt(pid))
}

func checkSearchHistory(t *testing.T, mongoURL, tok string, productID int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	mc, err := mongo.Open(ctx, mongoURL)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = mc.Close(context.Background()) })

	n, err := mongo.Collection[catalog.SearchEvent](mc, "search_history").Count(ctx, bson.D{{Key: "q", Value: tok}})
	if err != nil || n == 0 {
		t.Fatalf("search_history for %q: %d docs, err %v", tok, n, err)
	}

	views, err := mongo.Collection[catalog.ViewEvent](
		mc,
		"product_views",
	).Count(ctx, bson.D{{Key: "product_id", Value: productID}})
	// >= 1: product ids restart with each fresh Postgres run, Mongo keeps older runs.
	if err != nil || views < 1 {
		t.Fatalf("product_views for %d: %d docs, err %v", productID, views, err)
	}

	cur, err := mc.Database().Collection("search_history").Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var idx []bson.M
	if err := cur.All(ctx, &idx); err != nil {
		t.Fatal(err)
	}

	for _, i := range idx {
		if secs, ok := i["expireAfterSeconds"]; ok && toInt(secs) == 90*24*3600 {
			return
		}
	}

	t.Fatalf("no 90-day TTL index on search_history: %v", idx)
}

func toInt(v any) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	}

	n, err := strconv.ParseInt(fmt.Sprint(v), 10, 64)
	if err != nil {
		return -1
	}

	return n
}

// s3Env is the RustFS configuration (compose defaults, overridable).
func s3Env() map[string]string {
	envs := map[string]string{
		"MEDIA_DISK": "s3", "S3_ENDPOINT": "http://localhost:9200", "S3_REGION": "us-east-1",
		"S3_BUCKET": "marketplace", "S3_ACCESS_KEY_ID": "rustfsadmin", "S3_SECRET_ACCESS_KEY": "rustfsadmin",
		"S3_USE_PATH_STYLE": "true", "S3_INSECURE_HTTP": "true",
	}

	for k := range envs {
		if v := os.Getenv("TEST_" + k); v != "" {
			envs[k] = v
		}
	}

	return envs
}

// TestUploadRustFS stores an uploaded image's variants in the S3 bucket
// (RustFS locally) and serves them through the media route.
func TestUploadRustFS(t *testing.T) {
	envs := s3Env()

	u, err := url.Parse(envs["S3_ENDPOINT"])
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.DialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		t.Skipf("S3 endpoint %s unreachable: %v", u.Host, err)
	}

	_ = conn.Close()

	disk, err := catalog.OpenDisk(config.MapSource(envs), config.Development, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureBucket(t, disk, envs); err != nil {
		t.Skipf("bucket %s not usable: %v", envs["S3_BUCKET"], err)
	}

	app, svc := catalogApp(t, envs)
	owner := catalogSeller(t, svc, app, "SP")

	p, err := svc.CreateProduct(t.Context(), owner.ID, catalog.ProductInput{
		Title: "Upload " + word(6), PriceCents: 1000, Stock: 1, WeightGrams: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	var img bytes.Buffer
	src := image.NewRGBA(image.Rect(0, 0, 1600, 900))

	for y := range 900 {
		for x := range 1600 {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}

	if err := jpeg.Encode(&img, src, nil); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer

	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="images"; filename="photo.jpg"`)
	h.Set("Content-Type", "image/jpeg")

	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = part.Write(img.Bytes())
	_ = w.Close()

	authtest.ActingAs(t, app, app.Client(t).AcceptJSON(), owner).
		Do(http.MethodPost, "/seller/products/"+strconv.FormatInt(p.ID, 10)+"/images", &body, w.FormDataContentType()).
		AssertStatus(http.StatusSeeOther)

	sp, err := svc.SellerProduct(t.Context(), owner.ID, p.ID)
	if err != nil || len(sp.Images) != 1 {
		t.Fatalf("images after upload: %+v, %v", sp.Images, err)
	}

	im := sp.Images[0]
	if im.Processing {
		t.Fatal("image not processed (QUEUE_CONNECTION should be sync here)")
	}

	for width, u := range map[int]string{1280: im.URL1280, 640: im.URL640, 320: im.URL320} {
		obj := strings.TrimPrefix(u, "/media/")

		ok, err := disk.Exists(t.Context(), obj)
		if err != nil || !ok {
			t.Fatalf("object %s in bucket: %v %v", obj, ok, err)
		}

		res := app.Client(t).Get(u).AssertOK().AssertHeader("Content-Type", "image/jpeg")

		cfg, err := jpeg.DecodeConfig(strings.NewReader(res.Body()))
		if err != nil || cfg.Width != width {
			t.Fatalf("variant w%d: %dx%d %v", width, cfg.Width, cfg.Height, err)
		}
	}

	if err := svc.DeleteProductImage(t.Context(), owner.ID, p.ID, sp.Images[0].ID); err != nil {
		t.Fatal(err)
	}

	if ok, _ := disk.Exists(t.Context(), strings.TrimPrefix(im.URL640, "/media/")); ok {
		t.Fatal("variant still in the bucket after delete")
	}
}

// ensureBucket creates the bucket (a no-op when it exists) and checks it
// accepts writes.
func ensureBucket(t *testing.T, disk storage.Disk, envs map[string]string) error {
	t.Helper()

	cfg, err := config.Load[s3.Config](config.MapSource(envs))
	if err != nil {
		return err
	}

	s3disk, err := s3.New(cfg)
	if err != nil {
		return err
	}

	if err := s3disk.CreateBucket(t.Context()); err != nil {
		return err
	}

	p := "healthcheck/" + word(8) + ".txt"
	if err := storage.PutBytes(t.Context(), disk, p, []byte("ok")); err != nil {
		return err
	}

	if err := disk.Delete(t.Context(), p); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	return nil
}
