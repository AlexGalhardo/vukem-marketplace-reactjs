package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
)

func TestClientLogForwardsToTheClientLoggerService(t *testing.T) {
	t.Parallel()

	got := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		body["key"] = r.Header.Get("X-Client-Logger-Key")
		body["path"] = r.URL.Path
		got <- body

		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{
		Env: map[string]string{"CLIENT_LOGGER_URL": srv.URL, "CLIENT_LOGGER_KEY": "0123456789abcdef"},
	})
	c := app.Client(t)

	c.PostJSON("/client-log", map[string]any{"level": "error", "message": "boom", "url": "/cart"}).AssertStatus(204)

	select {
	case b := <-got:
		if b["path"] != "/log" || b["key"] != "0123456789abcdef" || b["message"] != "boom" || b["level"] != "error" {
			t.Fatalf("forwarded %v", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("report was not forwarded")
	}
}

func TestClientLogRejectsBadReports(t *testing.T) {
	t.Parallel()

	c := vukemtest.New(t, bootstrap.New).Client(t)

	for name, body := range map[string]map[string]any{
		"unknown level": {"level": "debug", "message": "x"},
		"empty message": {"level": "error", "message": " "},
		"huge message":  {"level": "error", "message": strings.Repeat("a", 501)},
	} {
		t.Logf("case %s", name)
		c.PostJSON("/client-log", body).AssertUnprocessable()
	}

	// Valid but nowhere to send it: accepted and dropped.
	c.PostJSON("/client-log", map[string]any{"level": "warn", "message": "x"}).AssertStatus(204)
}
