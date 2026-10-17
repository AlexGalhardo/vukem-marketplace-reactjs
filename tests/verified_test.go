package tests

import (
	"strings"
	"testing"

	vukem "github.com/AlexGalhardo/vukem"
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

// queuedMailJobs counts the queued (not yet sent) mail jobs. Since vukem
// v0.24.0 their payload is sealed with APP_KEY, so the recipient must not
// appear in clear text.
func queuedMailJobs(t *testing.T, fake *queue.Fake, email string) int {
	t.Helper()

	n := 0

	msgs := fake.Messages()
	for i := range msgs {
		m := &msgs[i]
		if m.Kind != vukem.MailJobKind {
			continue
		}

		if strings.Contains(string(m.Payload), email) {
			t.Fatalf("queued mail payload shows %s in clear text", email)
		}

		n++
	}

	return n
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

	if got := queuedMailJobs(t, fake, "vera@example.com"); got != 1 {
		t.Fatalf("queued verification mails = %d, want 1", got)
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

	if got := queuedMailJobs(t, fake, "vera@example.com"); got != 2 {
		t.Fatalf("queued verification mails after resend = %d, want 2", got)
	}

	runQueued(t, app, fake)
	c.Get(link(t, app, "vera@example.com")).AssertRedirect("/dashboard")

	c.Get("/test/verified").AssertOK().AssertSee("verified area")
	c.PostJSON("/test/verified", nil).AssertOK()
}
