package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/storage"
)

// BlobConfig configures the Vercel Blob disk (MEDIA_DISK=vercel-blob, or
// implied by BLOB_READ_WRITE_TOKEN being set). The token looks like
// vercel_blob_rw_<store id>_<secret>; the store id names the public host.
type BlobConfig struct {
	Token string `env:"BLOB_READ_WRITE_TOKEN"`
	// APIURL and PublicURL override the Vercel endpoints (tests).
	APIURL    string `env:"BLOB_API_URL" default:"https://blob.vercel-storage.com"`
	PublicURL string `env:"BLOB_PUBLIC_URL"`
}

var errBlobToken = errors.New("catalog: BLOB_READ_WRITE_TOKEN is missing or malformed (vercel_blob_rw_<store>_<secret>)")

// blobAPIVersion is the Vercel Blob REST API version the calls follow.
const blobAPIVersion = "7"

// VercelBlob is a storage.Disk over a public Vercel Blob store: objects are
// written at their exact path (no random suffix; keys already carry random
// names), read from the store's public host and never listed beyond List.
type VercelBlob struct {
	token  string
	api    string
	public string
	http   *http.Client
}

// NewVercelBlob returns the disk of cfg.
func NewVercelBlob(cfg BlobConfig) (*VercelBlob, error) {
	public := strings.TrimSuffix(cfg.PublicURL, "/")

	if public == "" {
		parts := strings.Split(cfg.Token, "_")
		if len(parts) < 5 || parts[0] != "vercel" || parts[1] != "blob" || parts[3] == "" {
			return nil, errBlobToken
		}

		public = "https://" + strings.ToLower(parts[3]) + ".public.blob.vercel-storage.com"
	}

	if cfg.Token == "" {
		return nil, errBlobToken
	}

	return &VercelBlob{
		token:  cfg.Token,
		api:    strings.TrimSuffix(cfg.APIURL, "/"),
		public: public,
		http:   &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}

	return strings.Join(parts, "/")
}

func (b *VercelBlob) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+b.token)
	req.Header.Set("x-api-version", blobAPIVersion)

	res, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vercel blob: %w", err)
	}

	return res, nil
}

// apiError turns a non-2xx API response into an error (and closes it).
func apiError(res *http.Response) error {
	defer res.Body.Close()

	msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))

	return fmt.Errorf("vercel blob: %s: %s", res.Status, strings.TrimSpace(string(msg)))
}

// Put implements storage.Disk. Images are capped at MaxImageBytes by the
// upload rules, so the body is buffered to send a Content-Length.
func (b *VercelBlob) Put(ctx context.Context, p string, r io.Reader) error {
	c, err := storage.Clean(p)
	if err != nil {
		return err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, b.api+"/"+escapePath(c), bytes.NewReader(data))
	if err != nil {
		return err
	}

	ctype := mime.TypeByExtension(path.Ext(c))
	if ctype == "" {
		ctype = "application/octet-stream"
	}

	req.Header.Set("x-content-type", ctype)
	req.Header.Set("x-add-random-suffix", "0")
	req.Header.Set("x-allow-overwrite", "1")
	req.Header.Set("x-cache-control-max-age", "31536000")

	res, err := b.do(req)
	if err != nil {
		return err
	}

	if res.StatusCode/100 != 2 {
		return apiError(res)
	}

	return res.Body.Close()
}

// Open implements storage.Disk.
func (b *VercelBlob) Open(ctx context.Context, p string) (io.ReadCloser, error) {
	c, err := storage.Clean(p)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL(c), nil)
	if err != nil {
		return nil, err
	}

	res, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vercel blob: %w", err)
	}

	switch {
	case res.StatusCode == http.StatusNotFound:
		res.Body.Close()

		return nil, &fs.PathError{Op: "open", Path: c, Err: fs.ErrNotExist}
	case res.StatusCode/100 != 2:
		return nil, apiError(res)
	}

	return res.Body, nil
}

// Exists implements storage.Disk.
func (b *VercelBlob) Exists(ctx context.Context, p string) (bool, error) {
	rc, err := b.Open(ctx, p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, rc.Close()
}

// Delete implements storage.Disk (missing files are not an error).
func (b *VercelBlob) Delete(ctx context.Context, p string) error {
	c, err := storage.Clean(p)
	if err != nil {
		return err
	}

	body, err := json.Marshal(map[string][]string{"urls": {b.URL(c)}})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.api+"/delete", bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := b.do(req)
	if err != nil {
		return err
	}

	if res.StatusCode/100 != 2 {
		return apiError(res)
	}

	return res.Body.Close()
}

// List implements storage.Disk.
func (b *VercelBlob) List(ctx context.Context, dir string) ([]string, error) {
	prefix := ""

	if dir != "" && dir != "." {
		c, err := storage.Clean(dir)
		if err != nil {
			return nil, err
		}

		prefix = c + "/"
	}

	var (
		out    []string
		cursor string
	)

	for {
		q := url.Values{"prefix": {prefix}, "limit": {"1000"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.api+"/?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}

		res, err := b.do(req)
		if err != nil {
			return nil, err
		}

		if res.StatusCode/100 != 2 {
			return nil, apiError(res)
		}

		var page struct {
			Blobs []struct {
				Pathname string `json:"pathname"`
			} `json:"blobs"`
			Cursor  string `json:"cursor"`
			HasMore bool   `json:"hasMore"`
		}

		err = json.NewDecoder(res.Body).Decode(&page)
		res.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("vercel blob: list: %w", err)
		}

		for _, bl := range page.Blobs {
			out = append(out, bl.Pathname)
		}

		if !page.HasMore || page.Cursor == "" {
			break
		}

		cursor = page.Cursor
	}

	sort.Strings(out)

	return out, nil
}

// URL implements storage.Disk: the public URL of the object.
func (b *VercelBlob) URL(p string) string { return b.public + "/" + escapePath(p) }
