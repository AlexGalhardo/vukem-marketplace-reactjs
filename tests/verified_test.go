package tests

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/mail"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/vukemtest"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

// queuedApp boots the app with a fake queue: jobs are recorded instead of
// running inline, so a test sees what left the request path. extra adds
// test-only routes.
func queuedApp(
	t *testing.T,
	env map[string]string,
	extra ...func(*router.Router, *vukem.App) error,
) (*vukemtest.App, *queue.Fake) {
	t.Helper()

	fake := queue.NewFake()
	opts := []vukem.Option{vukem.WithQueueDriver(fake)}

	for _, fn := range extra {
		opts = append(opts, vukem.WithRoutes(fn))
	}

	return vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: env, AppOptions: opts}), fake
}

// queuedMailTo returns the mails queued (not yet sent) to email.
func queuedMailTo(t *testing.T, fake *queue.Fake, email string) []mail.Message {
	t.Helper()

	var out []mail.Message

	msgs := fake.Messages()
	for i := range msgs {
		m := &msgs[i]
		if m.Kind != vukem.MailJobKind {
			continue
		}

		var msg mail.Message
		if err := json.Unmarshal(m.Payload, &msg); err != nil {
			t.Fatal(err)
		}

		if slices.ContainsFunc(
			msg.To,
			func(to string) bool { return strings.Contains(to, "<"+email+">") || to == email },
		) {
			out = append(out, msg)
		}
	}

	return out
}

// runQueued runs every job the fake recorded, like a worker would.
func runQueued(t *testing.T, app *vukemtest.App, fake *queue.Fake) {
	t.Helper()

	q, err := app.Queue()
	if err != nil {
		t.Fatal(err)
	}

	for {
		m, err := fake.Pop(t.Context(), []string{"default"})
		if err != nil {
			t.Fatal(err)
		}

		if m == nil {
			break
		}

		if err := q.Process(t.Context(), m); err != nil {
			t.Fatalf("job %s: %v", m.Kind, err)
		}
	}

	if n := fake.Pending(); n != 0 {
		t.Fatalf("%d jobs were released for a retry", n)
	}

	if failed := fake.Failed(); len(failed) > 0 {
		m := &failed[0]
		t.Fatalf("job %s failed: %s", m.Kind, m.LastError)
	}
}

// verifiedOnly registers /test/verified behind accounts.RequireVerified.
func verifiedOnly(r *router.Router, app *vukem.App) error {
	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	ok := func(c *web.Ctx) error { return c.String(200, "verified area") }
	r.Get("/test/verified", ok).Use(acc.RequireVerified)
	r.Post("/test/verified", ok).Use(acc.RequireVerified)

	return nil
}

func TestVerifiedMiddleware(t *testing.T) {
	t.Parallel()

	app, fake := queuedApp(t, nil, verifiedOnly)

	app.Client(t).Get("/test/verified").AssertRedirect("/login")

	c := app.Client(t).Inertia("")
	register(t, c, "vera@example.com")

	// The verification mail is queued, not sent during the request.
	if len(app.Mails().SentTo("vera@example.com")) != 0 {
		t.Fatal("verification mail was sent on the request path")
	}

	if got := queuedMailTo(t, fake, "vera@example.com"); len(got) != 1 {
		t.Fatalf("queued verification mails = %d, want 1", len(got))
	}

	// Unverified users are sent to the notice: Inertia visits, form posts
	// and plain page loads alike; JSON clients get 403.
	c.Get("/test/verified").AssertRedirect("/email/verify")
	c.PostJSON("/test/verified", nil).AssertRedirect("/email/verify")
	c.Get("/email/verify").AssertInertia("Auth/VerifyEmail")

	api := app.Client(t).WithCookie("vukem_session", c.CookieValue("vukem_session")).AcceptJSON()
	api.Get("/test/verified").AssertForbidden()

	// The resend button queues another mail.
	c.PostJSON("/email/verification-notification", nil).AssertRedirect("/email/verify")

	if got := queuedMailTo(t, fake, "vera@example.com"); len(got) != 2 {
		t.Fatalf("queued verification mails after resend = %d, want 2", len(got))
	}

	runQueued(t, app, fake)
	c.Get(link(t, app, "vera@example.com")).AssertRedirect("/dashboard")

	c.Get("/test/verified").AssertOK().AssertSee("verified area")
	c.PostJSON("/test/verified", nil).AssertOK()
}
