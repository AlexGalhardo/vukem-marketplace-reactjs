package tests

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

const telegramToken = "999:contact-test-token"

// telegramInbox is an httptest Bot API that keeps the texts it receives.
func telegramInbox(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		mu    sync.Mutex
		texts []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot"+telegramToken+"/sendMessage" {
			http.NotFound(w, r)
			return
		}

		var m struct {
			Text string `json:"text"`
		}

		_ = json.NewDecoder(r.Body).Decode(&m)

		mu.Lock()
		texts = append(texts, m.Text)
		mu.Unlock()
	}))
	t.Cleanup(srv.Close)

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()

		return append([]string(nil), texts...)
	}
}

func contactApp(t *testing.T, telegramURL string) *vukemtest.App {
	t.Helper()

	env := map[string]string{}
	if telegramURL != "" {
		env = map[string]string{
			"TELEGRAM_BOT_TOKEN": telegramToken,
			"TELEGRAM_CHAT_ID":   "-1",
			"TELEGRAM_API_URL":   telegramURL,
			"TELEGRAM_MIN_LEVEL": "ERROR+8",
		}
	}

	return vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: env})
}

func validContact() map[string]any {
	return map[string]any{
		"name":    "<b>Ana</b>",
		"email":   "ana@example.com",
		"subject": "Order question",
		"body":    "Where is my order? <script>alert(1)</script>",
	}
}

func TestContactFormStoresAndForwardsToTelegram(t *testing.T) {
	t.Parallel()

	srv, inbox := telegramInbox(t)
	app := contactApp(t, srv.URL)
	c := app.Client(t).Inertia("")

	c.Get("/contact").AssertInertia("Contact")
	c.PostJSON("/contact", validContact()).AssertRedirect("/contact")

	vukemtest.AssertDatabaseHas(
		t,
		app,
		"contact_messages",
		map[string]any{"email": "ana@example.com", "subject": "Order question"},
	)

	got := inbox()
	if len(got) != 1 {
		t.Fatalf("telegram got %d messages, want 1", len(got))
	}

	for _, want := range []string{"&lt;b&gt;Ana&lt;/b&gt;", "ana@example.com", "Order question", "&lt;script&gt;"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("message %q lacks %q (fields must be escaped)", got[0], want)
		}
	}

	if strings.Contains(got[0], "<script>") {
		t.Error("message carries unescaped HTML")
	}
}

func TestContactFormWorksWithoutTelegram(t *testing.T) {
	t.Parallel()

	app := contactApp(t, "")
	app.Client(t).Inertia("").PostJSON("/contact", validContact()).AssertRedirect("/contact")
	vukemtest.AssertDatabaseCount(t, app, "contact_messages", 1)
}

func TestContactFormValidation(t *testing.T) {
	t.Parallel()

	app := contactApp(t, "")
	c := app.Client(t).Inertia("")

	cases := map[string]map[string]any{
		"name":    {"name": "A"},
		"email":   {"email": "not-an-email"},
		"subject": {"subject": "x"},
		"body":    {"body": "short"},
	}

	for field, override := range cases {
		in := validContact()
		maps.Copy(in, override)

		rejected(t, c, "/contact", "/contact", "Contact", field, in)
	}

	vukemtest.AssertDatabaseCount(t, app, "contact_messages", 0)
}

func TestContactFormIsRateLimitedPerIP(t *testing.T) {
	t.Parallel()

	app := contactApp(t, "")
	c := app.Client(t).Inertia("")

	for range 5 {
		c.PostJSON("/contact", validContact()).AssertRedirect("/contact")
	}

	c.PostJSON("/contact", validContact()).AssertStatus(http.StatusTooManyRequests)
	vukemtest.AssertDatabaseCount(t, app, "contact_messages", 5)
}
