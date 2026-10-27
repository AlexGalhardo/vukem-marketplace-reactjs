package catalog

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/storage"
	"github.com/AlexGalhardo/vukem/storage/storagetest"
)

// fakeBlob is a minimal Vercel Blob: API and public host on one server.
func fakeBlob(t *testing.T) *httptest.Server {
	t.Helper()

	var (
		mu    sync.Mutex
		files = map[string][]byte{}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer vercel_blob_rw_store1_secret" && r.Method != http.MethodGet ||
			r.Method == http.MethodGet && r.URL.Path == "/" && r.Header.Get("Authorization") == "" {
			http.Error(w, "forbidden", http.StatusForbidden)

			return
		}

		mu.Lock()
		defer mu.Unlock()

		name := strings.TrimPrefix(r.URL.Path, "/")

		switch {
		case r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			if r.Header.Get("x-add-random-suffix") != "0" || r.Header.Get("x-allow-overwrite") != "1" || r.Header.Get("x-content-type") == "" {
				http.Error(w, "bad headers", http.StatusBadRequest)

				return
			}

			files[name] = b
			_, _ = w.Write([]byte(`{"pathname":"` + name + `"}`))
		case r.Method == http.MethodPost && name == "delete":
			var in struct{ URLs []string }
			_ = json.NewDecoder(r.Body).Decode(&in)

			for _, u := range in.URLs {
				p, _ := url.Parse(u)
				delete(files, strings.TrimPrefix(p.Path, "/"))
			}
		case r.Method == http.MethodGet && name == "":
			var blobs []map[string]string

			for p := range files {
				if strings.HasPrefix(p, r.URL.Query().Get("prefix")) {
					blobs = append(blobs, map[string]string{"pathname": p})
				}
			}

			sort.Slice(blobs, func(i, j int) bool { return blobs[i]["pathname"] < blobs[j]["pathname"] })
			_ = json.NewEncoder(w).Encode(map[string]any{"blobs": blobs, "hasMore": false})
		case r.Method == http.MethodGet:
			b, ok := files[name]
			if !ok {
				http.NotFound(w, r)

				return
			}

			_, _ = w.Write(b)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestVercelBlobDisk(t *testing.T) {
	t.Parallel()

	srv := fakeBlob(t)

	disk, err := NewVercelBlob(BlobConfig{Token: "vercel_blob_rw_store1_secret", APIURL: srv.URL, PublicURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	storagetest.Run(t, disk)
	storagetest.RunTraversal(t, disk)
}

func TestVercelBlobPublicURLFromToken(t *testing.T) {
	t.Parallel()

	disk, err := NewVercelBlob(BlobConfig{Token: "vercel_blob_rw_AbC123_secret", APIURL: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := disk.URL("products/1/a b.jpg"), "https://abc123.public.blob.vercel-storage.com/products/1/a%20b.jpg"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}

	for _, tok := range []string{"", "nope", "vercel_blob_rw__x"} {
		if _, err := NewVercelBlob(BlobConfig{Token: tok}); err == nil {
			t.Fatalf("token %q accepted", tok)
		}
	}
}

func TestOpenDiskPicksVercelBlob(t *testing.T) {
	t.Parallel()

	src := config.MapSource(map[string]string{"BLOB_READ_WRITE_TOKEN": "vercel_blob_rw_store1_secret"})

	disk, err := OpenDisk(src, config.Production, false)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := disk.(*VercelBlob); !ok {
		t.Fatalf("disk = %T, want *VercelBlob", disk)
	}

	if _, err := OpenDisk(config.MapSource(nil), config.Production, false); err == nil {
		t.Fatal("production without a remote disk must fail")
	}

	var _ storage.Disk = disk
}
