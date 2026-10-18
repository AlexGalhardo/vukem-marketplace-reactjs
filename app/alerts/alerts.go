// Package alerts is the Telegram ops channel: contact form messages and
// rate-limited 5xx alerts, both sent from queue jobs through vukem's
// app.Telegram. Without TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID every send
// is a no-op.
package alerts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/logging/telegram"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
)

// DefaultWindow is how long further alerts with the same fingerprint stay
// muted after one was sent.
const DefaultWindow = 10 * time.Minute

// SendJob delivers one pre-formatted alert.
type SendJob struct {
	Text string `json:"text"`
}

// Kind implements queue.Job.
func (SendJob) Kind() string { return "alerts.send" }

// ForwardContact forwards a stored contact message. The payload is only
// the row id, so no personal data sits in the queue.
type ForwardContact struct {
	MessageID int64 `json:"message_id"`
}

// Kind implements queue.Job.
func (ForwardContact) Kind() string { return "alerts.forward_contact" }

// Alert is one failed request. It deliberately holds no path values,
// query string, headers, body or user: only what identifies the route.
type Alert struct {
	Method    string
	Route     string
	Status    int
	RequestID string
}

// Fingerprint groups alerts of the same failure.
func (a Alert) Fingerprint() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s %s %d", a.Method, a.Route, a.Status)))
	return hex.EncodeToString(sum[:8])
}

// Notifier sends at most one alert per fingerprint per Window. Allow must
// be shared by every instance (Redis) for the limit to hold cluster-wide.
type Notifier struct {
	// Allow reports whether key may alert now and mutes it for ttl
	// (an atomic set-if-absent).
	Allow func(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Send delivers the formatted alert (usually by dispatching SendJob).
	Send   func(ctx context.Context, text string) error
	Window time.Duration
	App    string
	Env    string
	Log    *slog.Logger
}

// Notify formats a and sends it unless its fingerprint is muted. Limiter
// failures drop the alert (fail closed: an outage of the store must not
// turn every failing request into a chat message).
func (n *Notifier) Notify(ctx context.Context, a Alert) {
	window := n.Window
	if window <= 0 {
		window = DefaultWindow
	}

	fp := a.Fingerprint()

	ok, err := n.Allow(ctx, "alerts:"+fp, window)
	if err != nil {
		n.Log.WarnContext(ctx, "alerts: limiter unavailable, alert dropped", "err", err, "fingerprint", fp)
		return
	}

	if !ok {
		return
	}

	if err := n.Send(ctx, n.format(a, fp, window)); err != nil {
		n.Log.WarnContext(ctx, "alerts: send failed", "err", err, "fingerprint", fp)
	}
}

func (n *Notifier) format(a Alert, fp string, window time.Duration) string {
	route := a.Route
	if route == "" {
		route = "(no route)"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "🚨 <b>HTTP %d</b> · %s (%s)\n", a.Status, html.EscapeString(n.App), html.EscapeString(n.Env))
	fmt.Fprintf(&b, "<code>%s %s</code>\n", html.EscapeString(a.Method), html.EscapeString(route))

	if a.RequestID != "" {
		fmt.Fprintf(&b, "request_id: <code>%s</code>\n", html.EscapeString(a.RequestID))
	}

	fmt.Fprintf(&b, "fingerprint: <code>%s</code> (muted for %s)", fp, window)

	return b.String()
}

type routeKey struct{}

// Middleware alerts on 5xx responses (panics included: vukem's Recover
// sits inside it and renders them as 500). Install it with UseServer and
// Route with Use, so the matched ServeMux pattern names the alert.
func (n *Notifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := new(string)
		rw, ok := w.(*web.ResponseWriter)
		if !ok {
			rw = &web.ResponseWriter{ResponseWriter: w}
		}

		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), routeKey{}, route)))

		if rw.Status() < http.StatusInternalServerError {
			return
		}

		// The request may be gone; the alert must still go out.
		ctx := context.WithoutCancel(r.Context())
		n.Notify(ctx, Alert{
			Method: r.Method, Route: *route, Status: rw.Status(), RequestID: web.RequestID(r.Context()),
		})
	})
}

// Route records the matched pattern for Middleware.
func (n *Notifier) Route(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := r.Context().Value(routeKey{}).(*string); ok && *p == "" {
			*p = routeOf(r.Pattern)
		}

		next.ServeHTTP(w, r)
	})
}

// routeOf turns "GET /p/{slug}" into "/p/{slug}".
func routeOf(pattern string) string {
	if _, rest, ok := strings.Cut(pattern, " "); ok {
		pattern = rest
	}

	if i := strings.IndexByte(pattern, '/'); i >= 0 {
		return strings.TrimSuffix(pattern[i:], "{$}")
	}

	return ""
}

// ContactMessage is what ForwardContact sends.
type ContactMessage struct {
	Name, Email, Subject, Body string
}

// FormatContact renders m as Telegram HTML; every field is escaped and
// the body is cut so the message fits Telegram's limit.
func FormatContact(m ContactMessage) string {
	return fmt.Sprintf("📬 <b>Contact</b>\n<b>From:</b> %s &lt;%s&gt;\n<b>Subject:</b> %s\n\n%s",
		html.EscapeString(m.Name), html.EscapeString(m.Email), html.EscapeString(m.Subject),
		html.EscapeString(truncate(m.Body, 3000)))
}

// LoadContact reads a stored contact message by id.
type LoadContact func(ctx context.Context, id int64) (ContactMessage, error)

// dbLoader reads contact_messages through the app database, opened on
// first use.
func dbLoader(app *vukem.App) LoadContact {
	open := sync.OnceValues(app.DB)

	return func(ctx context.Context, id int64) (ContactMessage, error) {
		db, err := open()
		if err != nil {
			return ContactMessage{}, err
		}

		m, err := orm.QueryOf[models.ContactMessage](db).Find(ctx, id)
		if err != nil {
			return ContactMessage{}, err
		}

		return ContactMessage{Name: m.Name, Email: m.Email, Subject: m.Subject, Body: m.Body}, nil
	}
}

// Provider registers the Telegram jobs and, when Telegram is configured,
// the 5xx alert middleware. Register it before the route providers.
type Provider struct {
	// Window overrides DefaultWindow.
	Window time.Duration
}

// Register implements vukem.Provider.
func (p Provider) Register(app *vukem.App) error {
	tg := app.Telegram

	q, err := app.Queue()
	if err != nil {
		return err
	}

	if err := q.Register(
		queue.Handle(func(ctx context.Context, j SendJob) error { return send(ctx, tg, j.Text) },
			queue.Retries(3), queue.Backoff(queue.Exponential(10*time.Second))),
		queue.Handle(forwardHandler(tg, dbLoader(app)),
			queue.Retries(5), queue.Backoff(queue.Exponential(30*time.Second))),
	); err != nil {
		return fmt.Errorf("alerts: %w", err)
	}

	if tg == nil {
		return nil
	}

	n := &Notifier{
		Allow:  cacheAllow(app),
		Send:   func(ctx context.Context, text string) error { return q.Dispatch(ctx, SendJob{Text: text}) },
		Window: p.Window,
		App:    app.Config.Name,
		Env:    string(app.Config.Env),
		Log:    app.Log,
	}

	app.Router.UseServer(n.Middleware)
	app.Router.Use(n.Route)

	return nil
}

// send delivers htmlText now and returns the delivery error, so the job
// retries a lost message; it is a no-op when Telegram is not configured.
func send(ctx context.Context, tg *telegram.Handler, htmlText string) error {
	if tg == nil {
		return nil
	}

	return tg.SendSync(ctx, htmlText)
}

func forwardHandler(tg *telegram.Handler, load LoadContact) func(ctx context.Context, j ForwardContact) error {
	return func(ctx context.Context, j ForwardContact) error {
		if tg == nil {
			return nil
		}

		m, err := load(ctx, j.MessageID)
		if err != nil {
			return fmt.Errorf("load contact message %d: %w", j.MessageID, err)
		}

		return send(ctx, tg, FormatContact(m))
	}
}

// cacheAllow mutes keys in the app cache, which is Redis when REDIS_URL is
// set (shared by every instance) and per-process memory otherwise. The
// cache is opened on the first alert, not at boot.
func cacheAllow(app *vukem.App) func(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	store := sync.OnceValues(app.Cache)

	return func(ctx context.Context, key string, ttl time.Duration) (bool, error) {
		c, err := store()
		if err != nil {
			return false, err
		}

		return c.Store().Add(ctx, key, []byte("1"), ttl)
	}
}

// truncate cuts s to at most n runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}

	return string(r[:n-1]) + "…"
}
