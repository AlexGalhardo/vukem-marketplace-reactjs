package catalog_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/storage"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/catalog"
)

// TestThumbnailJob: an upload shows its original until the queued job
// makes the variants; the job is idempotent and drops stale work.
func TestThumbnailJob(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, _ := seller(t, app, "thumb@example.com", "Loja Thumb", "SP")
	p := product(t, app, u, productSpec{title: "Foto Pendente", price: 1000, stock: 1})

	fake := queue.NewFake()
	disk := storage.NewMemory("")

	q := queue.New(fake, slog.New(slog.DiscardHandler))

	svc, err := catalog.New(catalog.Config{DB: mustDB(t, app), Disk: disk, Queue: q})
	if err != nil {
		t.Fatal(err)
	}

	if err := q.Register(svc.ThumbnailRegistration()); err != nil {
		t.Fatal(err)
	}

	img, err := svc.SaveProductImage(t.Context(), u.ID, p.ID, "photo.png", bytes.NewReader(pngBytes(t, 900, 300)))
	if err != nil {
		t.Fatal(err)
	}

	original := strings.TrimPrefix(img.URL640, "/media/")
	if !img.Processing || img.URL320 != img.URL1280 || !strings.HasSuffix(original, ".png") {
		t.Fatalf("pending image = %+v, want the original at every width", img)
	}

	jobs := queue.Dispatched[catalog.ThumbnailJob](fake)
	if len(jobs) != 1 || jobs[0].Target != catalog.ThumbProduct || jobs[0].ID != img.ID {
		t.Fatalf("dispatched = %+v", jobs)
	}

	for range 2 { // a retried delivery is a no-op
		if err := svc.ProcessThumbnails(t.Context(), jobs[0]); err != nil {
			t.Fatal(err)
		}
	}

	sp, err := svc.SellerProduct(t.Context(), u.ID, p.ID)
	if err != nil {
		t.Fatal(err)
	}

	done := sp.Images[0]
	if done.Processing || done.Width != 900 || done.Height != 300 || done.URL320 == done.URL1280 {
		t.Fatalf("processed image = %+v", done)
	}

	for _, url := range []string{done.URL320, done.URL640, done.URL1280} {
		if ok, _ := disk.Exists(t.Context(), strings.TrimPrefix(url, "/media/")); !ok {
			t.Fatalf("variant %s missing", url)
		}
	}

	if ok, _ := disk.Exists(t.Context(), original); ok {
		t.Fatal("original kept after processing")
	}

	// A job for a deleted image does nothing.
	img2, err := svc.SaveProductImage(t.Context(), u.ID, p.ID, "b.jpg", bytes.NewReader(jpegBytes(t, 50, 50)))
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteProductImage(t.Context(), u.ID, p.ID, img2.ID); err != nil {
		t.Fatal(err)
	}

	stale := queue.Dispatched[catalog.ThumbnailJob](fake)[1]
	if err := svc.ProcessThumbnails(t.Context(), stale); err != nil {
		t.Fatalf("stale job: %v", err)
	}

	// Forged jobs cannot point the worker at arbitrary objects.
	if err := svc.ProcessThumbnails(t.Context(), catalog.ThumbnailJob{
		Target: catalog.ThumbProduct, ID: img.ID, Src: "../../.env",
	}); err == nil {
		t.Fatal("job with a forged source accepted")
	}
}
