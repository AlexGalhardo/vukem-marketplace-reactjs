package alerts_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/logging/telegram"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/vukemtest"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/alerts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

const token = "123456:SECRET-bot-token"

type sent struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// fakeTelegram is an httptest Bot API that records sendMessage calls.
type fakeTelegram struct {
	*httptest.Server

	mu     sync.Mutex
	msgs   []sent
	status int
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	t.Helper()

	f := &fakeTelegram{status: http.StatusOK}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/bot"+token+"/sendMessage" {
			http.NotFound(w, r)
			return
		}

		var m sent
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		f.mu.Lock()
		f.msgs = append(f.msgs, m)
		status := f.status
		f.mu.Unlock()

		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(f.Close)

	return f
}

func (f *fakeTelegram) messages() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]sent(nil), f.msgs...)
}

// telegramApp boots the app with Telegram pointed at url; log records are
// not forwarded (TELEGRAM_LOG=false), so only the alerts jobs post.
func telegramApp(t *testing.T, url string, opts ...vukem.Option) *vukemtest.App {
	t.Helper()

	return vukemtest.New(t, bootstrap.New, vukemtest.Options{
		Env: map[string]string{
			"TELEGRAM_BOT_TOKEN": token,
			"TELEGRAM_CHAT_ID":   "-100",
			"TELEGRAM_LOG":       "false",
		},
		AppOptions: append(opts, vukem.WithTelegram(func(c *telegram.Config) { c.BaseURL = url })),
	})
}

func dispatch(t *testing.T, app *vukemtest.App, j queue.Job) error {
	t.Helper()

	q, err := app.Queue()
	if err != nil {
		t.Fatal(err)
	}

	return q.Dispatch(t.Context(), j)
}

func TestSendJobDeliversSynchronously(t *testing.T) {
	t.Parallel()

	f := newFakeTelegram(t)

	if err := dispatch(t, telegramApp(t, f.URL), alerts.SendJob{Text: "<b>hi</b>"}); err != nil {
		t.Fatal(err)
	}

	got := f.messages()
	if len(got) != 1 || got[0] != (sent{ChatID: "-100", Text: "<b>hi</b>", ParseMode: "HTML"}) {
		t.Fatalf("sent %+v", got)
	}
}

// The job must see the delivery error, or the queue could never retry a
// lost alert; the error must not carry the bot token.
func TestSendJobReturnsTheDeliveryErrorWithoutTheToken(t *testing.T) {
	t.Parallel()

	f := newFakeTelegram(t)
	f.status = http.StatusInternalServerError

	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()

	for _, url := range []string{f.URL, down.URL} {
		err := dispatch(t, telegramApp(t, url), alerts.SendJob{Text: "x"})
		if err == nil {
			t.Fatalf("send to %s succeeded, want an error", url)
		}

		if strings.Contains(err.Error(), "SECRET") {
			t.Errorf("error leaks the bot token: %v", err)
		}
	}
}

func TestJobsAreNoOpsWithoutTelegram(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	for _, j := range []queue.Job{alerts.SendJob{Text: "x"}, alerts.ForwardContact{MessageID: 1}} {
		if err := dispatch(t, app, j); err != nil {
			t.Errorf("%s without Telegram: %v", j.Kind(), err)
		}
	}
}

// muter is an Allow func with a fake clock.
type muter struct {
	mu    sync.Mutex
	now   time.Time
	until map[string]time.Time
}

func (m *muter) allow(_ context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.now.Before(m.until[key]) {
		return false, nil
	}

	m.until[key] = m.now.Add(ttl)

	return true, nil
}

func TestNotifierSendsOneAlertPerFingerprintPerWindow(t *testing.T) {
	t.Parallel()

	m := &muter{now: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC), until: map[string]time.Time{}}

	var texts []string

	n := &alerts.Notifier{
		Allow:  m.allow,
		Send:   func(_ context.Context, text string) error { texts = append(texts, text); return nil },
		Window: 10 * time.Minute,
		App:    "Market<script>",
		Env:    "production",
	}

	boom := alerts.Alert{Method: "GET", Route: "/p/{slug}", Status: 500, RequestID: "r1"}

	steps := []struct {
		advance time.Duration
		alert   alerts.Alert
		want    int
	}{
		{0, boom, 1},
		{time.Minute, boom, 1}, // muted
		{0, alerts.Alert{Method: "POST", Route: "/p/{slug}", Status: 500}, 2}, // other fingerprint
		{0, alerts.Alert{Method: "GET", Route: "/p/{slug}", Status: 503}, 3},  // other status
		{10 * time.Minute, boom, 4}, // window over
	}

	for i, s := range steps {
		m.now = m.now.Add(s.advance)
		n.Notify(t.Context(), s.alert)

		if len(texts) != s.want {
			t.Fatalf("step %d: %d alerts, want %d", i, len(texts), s.want)
		}
	}

	if !strings.Contains(texts[0], "<code>GET /p/{slug}</code>") ||
		!strings.Contains(texts[0], "Market&lt;script&gt;") {
		t.Errorf("alert text not formatted/escaped: %q", texts[0])
	}
}

func TestNotifierFailsClosedWhenTheLimiterIsDown(t *testing.T) {
	t.Parallel()

	calls := 0
	n := &alerts.Notifier{
		Allow: func(context.Context, string, time.Duration) (bool, error) { return false, errors.New("redis down") },
		Send:  func(context.Context, string) error { calls++; return nil },
		Log:   vukemtest.New(t, bootstrap.New).Log,
	}

	n.Notify(t.Context(), alerts.Alert{Method: "GET", Route: "/", Status: 500})

	if calls != 0 {
		t.Fatalf("sent %d alerts with the limiter down, want 0", calls)
	}
}

func TestServerErrorsAlertTelegramOnceWithoutRequestData(t *testing.T) {
	t.Parallel()

	f := newFakeTelegram(t)

	app := telegramApp(t, f.URL, vukem.WithRoutes(func(r *router.Router, _ *vukem.App) error {
		r.Post("/boom/{id}", func(*web.Ctx) error { panic("db password is hunter2") })
		r.Get("/fail", func(*web.Ctx) error { return errors.New("upstream said hunter2") })

		return nil
	}))

	c := app.Client(t).WithHeader("Authorization", "Bearer hunter2-token")

	for range 3 {
		c.PostJSON("/boom/4242?token=hunter2", map[string]string{"card": "hunter2-body"}).
			AssertStatus(http.StatusInternalServerError)
	}

	c.Get("/missing").AssertStatus(http.StatusNotFound)
	c.Get("/fail").AssertStatus(http.StatusInternalServerError)

	got := f.messages()
	if len(got) != 2 {
		t.Fatalf("got %d Telegram messages, want 2 (one per failing route): %+v", len(got), got)
	}

	if !strings.Contains(got[0].Text, "POST /boom/{id}") || !strings.Contains(got[0].Text, "HTTP 500") {
		t.Errorf("first alert does not name the route: %q", got[0].Text)
	}

	for _, m := range got {
		for _, leak := range []string{"hunter2", "4242", "Bearer", "card"} {
			if strings.Contains(m.Text, leak) {
				t.Errorf("alert leaks %q: %q", leak, m.Text)
			}
		}
	}
}
