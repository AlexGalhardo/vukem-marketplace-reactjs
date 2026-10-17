package mailtrap

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fakeSandbox(t *testing.T, polls *atomic.Int32) *Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sandboxes/42/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Api-Token") != "tok" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// The message "arrives" on the second poll.
		if polls.Add(1) < 2 {
			_, _ = w.Write([]byte(`[]`))
			return
		}

		_, _ = w.Write(
			[]byte(
				`[{"id":7,"subject":"Verify your email address","to_email":"ana@example.com","sent_at":"2026-10-17T10:00:00Z"}]`,
			),
		)
	})
	mux.HandleFunc("GET /api/sandboxes/42/messages/7/body.html", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(
			[]byte(`<p><a href="https://app.test/email/verify/1?expires=9&amp;signature=abc">Verify</a></p>`),
		)
	})
	mux.HandleFunc("PATCH /api/sandboxes/42/clean", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":42}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &Client{BaseURL: srv.URL, Token: "tok", SandboxID: "42", Poll: time.Millisecond}
}

func TestWaitForAndFindLink(t *testing.T) {
	t.Parallel()

	var polls atomic.Int32
	c := fakeSandbox(t, &polls)

	msg, err := c.WaitFor(t.Context(), "ANA@example.com", "Verify", time.Second)
	if err != nil {
		t.Fatalf("WaitFor() err = %v", err)
	}

	html, err := c.HTML(t.Context(), msg.ID)
	if err != nil {
		t.Fatalf("HTML() err = %v", err)
	}

	link, ok := FindLink(html, "/email/verify/")
	if want := "https://app.test/email/verify/1?expires=9&signature=abc"; !ok || link != want {
		t.Fatalf("FindLink() = %q, %v; want %q", link, ok, want)
	}

	if err := c.Clean(t.Context()); err != nil {
		t.Fatalf("Clean() err = %v", err)
	}
}

func TestWaitForTimesOut(t *testing.T) {
	t.Parallel()

	var polls atomic.Int32
	polls.Store(-1000) // never delivers within the timeout
	c := fakeSandbox(t, &polls)

	if _, err := c.WaitFor(t.Context(), "ana@example.com", "Verify", 20*time.Millisecond); !errors.Is(err, ErrTimeout) {
		t.Fatalf("WaitFor() err = %v, want ErrTimeout", err)
	}
}

func TestBadTokenIsAnError(t *testing.T) {
	t.Parallel()

	var polls atomic.Int32
	c := fakeSandbox(t, &polls)
	c.Token = "wrong"

	if _, err := c.Messages(t.Context(), ""); err == nil {
		t.Fatal("Messages() with a bad token succeeded")
	}
}
