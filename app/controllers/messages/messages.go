// Package messages serves the buyer ↔ seller chat: the inboxes
// (/messages, /seller/messages), a conversation page, the JSON endpoints
// the page calls (send, catch-up, read receipts, typing), the realtime
// transports (WebSocket, SSE fallback) and the admin's read-only view.
// The chat service scopes every read and write to the participants.
package messages

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/broadcast"
	"github.com/AlexGalhardo/vukem/drivers/websocket"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/sse"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/chat"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/mail"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
)

// Socket limits.
const (
	// MaxConnections is how many sockets (or SSE streams) one user may
	// keep open on an instance: a few tabs, not a flood.
	MaxConnections = 5
	// MaxLifetime ends a socket after this long; the page reconnects and
	// catches up by message id, so a forgotten tab does not hold a
	// connection (and a session check) forever.
	MaxLifetime = 30 * time.Minute
	// ReadLimit caps one client frame (subscribe/unsubscribe only).
	ReadLimit = 4 << 10
)

// Options configures FromApp.
type Options struct {
	// Broker fans broadcast events out to every instance (nil: this
	// process only) and Prefix namespaces its channels.
	Broker sse.Broker
	Prefix string
	// NotifyAfter is the away time before unread messages are emailed.
	NotifyAfter time.Duration
	// MaxConnections overrides the per-user socket cap (0: default).
	MaxConnections int
}

// Handlers serves the chat.
type Handlers struct {
	svc   *chat.Service
	acc   *accounts.Accounts
	b     *broadcast.Server
	conns *chat.Connections
	ws    web.HandlerFunc
}

// FromApp builds the chat service, its broadcast server, the
// notification job (registered on app.Queue()) and the handlers.
func FromApp(app *vukem.App, acc *accounts.Accounts, o Options) (*Handlers, error) {
	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	c, err := app.Cache()
	if err != nil {
		return nil, err
	}

	q, err := app.Queue()
	if err != nil {
		return nil, err
	}

	notify := func(ctx context.Context, job chat.NotifyJob, delay time.Duration) error {
		return q.Dispatch(ctx, job, queue.Delay(delay), queue.Retries(3))
	}

	svc, err := chat.New(chat.Config{
		DB: db, Cache: c, Notify: notify, NotifyAfter: o.NotifyAfter, Log: app.Log, Now: app.Now,
	})
	if err != nil {
		return nil, err
	}

	b, err := chat.NewBroadcaster(o.Broker, o.Prefix, svc)
	if err != nil {
		return nil, err
	}

	svc.SetPublisher(b)
	app.OnShutdown(func(context.Context) error { b.Close(); return nil })

	base := strings.TrimSuffix(app.Config.URL, "/")
	sendMail := func(ctx context.Context, m chat.NotifyMail) error {
		return app.MailLater(ctx, mail.ChatMessage{
			To: m.To.Email, Name: chat.FirstName(m.To.Name), From: m.From, Unread: m.Unread, Preview: m.Preview,
			Link:    base + "/messages/" + strconv.FormatInt(m.ConversationID, 10),
			Subject: i18n.T(ctx, "chat.mail.subject", "from", m.From),
		}, queue.Retries(3), queue.Backoff(queue.Exponential(10*time.Second)))
	}

	err = q.Register(queue.Handle(func(ctx context.Context, job chat.NotifyJob) error {
		_, err := svc.HandleNotify(localized(ctx, app, job.Locale), job, sendMail)
		return err
	}))
	if err != nil {
		return nil, err
	}

	maxConns := o.MaxConnections
	if maxConns <= 0 {
		maxConns = MaxConnections
	}

	return &Handlers{
		svc: svc, acc: acc, b: b, conns: &chat.Connections{Max: maxConns},
		// Same-origin upgrades only (the driver's default): the socket
		// authenticates with the session cookie.
		ws: websocket.Broadcast(b, websocket.ReadLimit(ReadLimit)),
	}, nil
}

// localized returns ctx speaking locale, so a mail rendered by a worker
// uses the language of the request that queued it.
func localized(ctx context.Context, app *vukem.App, locale string) context.Context {
	if app.I18n == nil {
		return ctx
	}

	if l, ok := app.I18n.Match(locale); ok {
		locale = l
	} else {
		locale = app.I18n.Default()
	}

	return i18n.WithLocale(i18n.WithBundle(ctx, app.I18n), locale)
}

// Service returns the chat service.
func (h *Handlers) Service() *chat.Service { return h.svc }

// Broadcaster returns the broadcast server.
func (h *Handlers) Broadcaster() *broadcast.Server { return h.b }

// ThrottleKey keys the chat rate limits by signed-in user ("" skips them;
// the routes require a session anyway).
func ThrottleKey(r *http.Request) string {
	u, err := auth.UserFrom[models.User](r.Context(), "web")
	if err != nil {
		return ""
	}

	return "chat:" + u.AuthID()
}

// Localized translates errors whose messages are "chat.*" catalog keys
// into the request locale.
func Localized(fn func(*web.Ctx) error) func(*web.Ctx) error {
	return func(c *web.Ctx) error { return localize(c.Context(), fn(c)) }
}

func localize(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if verr, ok := errors.AsType[*errs.ValidationError](err); ok {
		out := errs.NewValidation()

		for field, msgs := range verr.Fields {
			for _, m := range msgs {
				_ = out.Add(field, translate(ctx, m))
			}
		}

		return out
	}

	if he, ok := errors.AsType[*errs.HTTPError](err); ok && strings.HasPrefix(he.Message, "chat.") {
		// A copy: the sentinel errors are shared package variables.
		return &errs.HTTPError{Status: he.Status, Message: i18n.T(ctx, he.Message), Err: he.Err, Headers: he.Headers}
	}

	return err
}

func translate(ctx context.Context, msg string) string {
	if strings.HasPrefix(msg, "chat.") {
		return i18n.T(ctx, msg)
	}

	return msg
}

func (h *Handlers) user(c *web.Ctx) (models.User, error) {
	u, err := h.acc.Web.User(c)
	if err != nil {
		return models.User{}, errs.ErrUnauthorized
	}

	return u, nil
}

func conversationID(c *web.Ctx) (int64, error) {
	id, err := c.ParamInt("id")
	if err != nil || id <= 0 {
		return 0, chat.ErrNotFound
	}

	return id, nil
}

// ChatEnabled reports whether the chat flag is on. It and RequireChat are
// the only readers of the flag, so moving it to another flags store is a
// one-line change.
func (h *Handlers) ChatEnabled(ctx context.Context) bool { return h.acc.Flags.Enabled(ctx, flags.Chat) }

// RequireChat serves next only while the chat flag is on (404 otherwise).
func (h *Handlers) RequireChat(next http.Handler) http.Handler {
	return h.acc.Flags.Require(flags.Chat)(next)
}

// Share shares chat.enabled and the signed-in user's unread count (header
// badge) with every page; both are computed only when a page renders.
// Rendering a page also counts as being around (last-seen).
func (h *Handlers) Share(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := web.NewCtx(w, r, web.ServicesFrom(r.Context()))

		inertia.Share(r.Context(), "chat", func(ctx context.Context) (any, error) {
			enabled := h.ChatEnabled(ctx)

			u, err := h.acc.Web.User(c)
			if err != nil || !enabled {
				return map[string]any{"enabled": enabled, "unread": 0}, nil //nolint:nilerr // signed out
			}

			h.svc.Touch(ctx, u.ID)

			n, err := h.svc.UnreadTotal(ctx, u.ID)
			if err != nil {
				n = 0 // a badge is not worth failing the page
			}

			return map[string]any{"enabled": true, "unread": n}, nil
		})

		next.ServeHTTP(w, r)
	})
}

// Index renders the buyer inbox (GET /messages).
func (h *Handlers) Index(c *web.Ctx) error { return h.inbox(c, chat.RoleBuyer) }

// SellerIndex renders the shop's inbox (GET /seller/messages).
func (h *Handlers) SellerIndex(c *web.Ctx) error { return h.inbox(c, chat.RoleSeller) }

func (h *Handlers) inbox(c *web.Ctx, role string) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	page, err := h.svc.Inbox(c.Context(), u.ID, role, c.QueryInt("page", 1))
	if errors.Is(err, chat.ErrNoShop) {
		return c.Redirect("/sell")
	}

	if err != nil {
		return err
	}

	return inertia.Render(c, "Messages/Index", inertia.Props{"role": role, "conversations": page, "me": u.ID})
}

// Start opens (or reopens) the conversation about a product or with a
// shop (POST /messages) and goes to it.
func (h *Handlers) Start(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, chat.StartSchema)
	if err != nil {
		return err
	}

	var conv chat.Conversation

	switch {
	case in.Product != "" && in.Shop == "":
		conv, err = h.svc.StartWithProduct(c.Context(), u.ID, in.Product)
	case in.Shop != "" && in.Product == "":
		conv, err = h.svc.StartWithShop(c.Context(), u.ID, in.Shop)
	default:
		return chat.ErrNotFound
	}

	if err != nil {
		return err
	}

	return c.Redirect("/messages/" + strconv.FormatInt(conv.ID, 10))
}

// Show renders a conversation (GET /messages/{id}) and marks it read.
func (h *Handlers) Show(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := conversationID(c)
	if err != nil {
		return err
	}

	ctx := c.Context()

	t, role, err := h.svc.Participant(ctx, u.ID, id)
	if err != nil {
		return err
	}

	if t.LastMessageID > 0 {
		read, err := h.svc.MarkRead(ctx, u.ID, id, t.LastMessageID)
		if err != nil {
			return err
		}

		if role == chat.RoleBuyer {
			t.BuyerLastReadID = read
		} else {
			t.SellerLastReadID = read
		}
	}

	detail, err := h.svc.Describe(ctx, t, role)
	if err != nil {
		return err
	}

	msgs, err := h.svc.Recent(ctx, id)
	if err != nil {
		return err
	}

	h.svc.Touch(ctx, u.ID)

	return inertia.Render(c, "Messages/Show", inertia.Props{
		"conversation": detail,
		"messages":     msgs,
		"me":           u.ID,
		"online":       h.svc.Online(ctx, t.Counterpart(u.ID)),
		"channels": map[string]string{
			"chat":     chat.MessagesChannel(id),
			"presence": chat.PresenceChannel(id),
		},
		"limits": map[string]int{"max": chat.MessageMax},
	})
}

// Send stores a message (POST /messages/{id}) and answers it as JSON.
func (h *Handlers) Send(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := conversationID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, chat.SendSchema)
	if err != nil {
		return err
	}

	msg, err := h.svc.Send(c.Context(), u.ID, id, in.Body)
	if err != nil {
		return err
	}

	h.svc.Touch(c.Context(), u.ID)

	return c.JSON(http.StatusCreated, map[string]any{"message": msg})
}

// Since returns the messages after ?after=<id> (GET /messages/{id}/since):
// the catch-up after a reconnect and the polling fallback.
func (h *Handlers) Since(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := conversationID(c)
	if err != nil {
		return err
	}

	after, err := strconv.ParseInt(c.Query("after"), 10, 64)
	if err != nil || after < 0 {
		after = 0
	}

	msgs, t, err := h.svc.Since(c.Context(), u.ID, id, after)
	if err != nil {
		return err
	}

	theirs := t.SellerLastReadID
	if t.Role(u.ID) == chat.RoleSeller {
		theirs = t.BuyerLastReadID
	}

	h.svc.Touch(c.Context(), u.ID)

	return c.JSON(http.StatusOK, map[string]any{
		"messages": msgs, "their_last_read_id": theirs, "online": h.svc.Online(c.Context(), t.Counterpart(u.ID)),
	})
}

// Read moves the read receipt (POST /messages/{id}/read) and returns the
// receipt and the user's unread total.
func (h *Handlers) Read(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := conversationID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, chat.ReadSchema)
	if err != nil {
		return err
	}

	read, err := h.svc.MarkRead(c.Context(), u.ID, id, in.LastID)
	if err != nil {
		return err
	}

	unread, err := h.svc.UnreadTotal(c.Context(), u.ID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{"last_read_id": read, "unread": unread})
}

// Typing tells the other side the user is typing (POST /messages/{id}/typing).
//
// workaround(vukem): the broadcast WebSocket protocol only takes
// subscribe/unsubscribe frames (no client events like Echo's whisper), so
// typing goes through this endpoint and a server broadcast.
func (h *Handlers) Typing(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := conversationID(c)
	if err != nil {
		return err
	}

	if err := h.svc.Typing(c.Context(), u.ID, id); err != nil {
		return err
	}

	return c.NoContent()
}

// Socket serves the broadcast WebSocket (GET /broadcast/ws): same-origin
// upgrades only, MaxConnections per user, MaxLifetime per socket; every
// subscription is authorized by the channel rules. While it is open the
// user counts as online.
func (h *Handlers) Socket(c *web.Ctx) error { return h.transport(c, h.ws) }

// Stream serves the same channels over server-sent events
// (GET /broadcast/sse?channels=a,b), the fallback when WebSockets are
// blocked.
func (h *Handlers) Stream(c *web.Ctx) error { return h.transport(c, h.b.SSE) }

func (h *Handlers) transport(c *web.Ctx, serve web.HandlerFunc) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	if !h.conns.Acquire(u.ID) {
		web.SecurityEvent(c.Request(), "chat.connections_exceeded", "user_id", u.ID)
		return errs.New(http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
	}
	defer h.conns.Release(u.ID)

	ctx, cancel := context.WithTimeout(c.Context(), MaxLifetime)
	defer cancel()

	c.SetRequest(c.Request().WithContext(ctx))

	seen, stop := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		h.svc.KeepSeen(seen, u.ID)
	}()

	defer func() { stop(); <-done }()

	return serve(c)
}

// AdminIndex lists every conversation (GET /admin/messages), read-only.
func (h *Handlers) AdminIndex(c *web.Ctx) error {
	page, err := h.svc.AdminInbox(c.Context(), c.QueryInt("page", 1))
	if err != nil {
		return err
	}

	return inertia.Render(c, "Admin/Messages", inertia.Props{"conversations": page})
}

// AdminShow shows a conversation read-only (GET /admin/messages/{id});
// every view is audited.
func (h *Handlers) AdminShow(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := conversationID(c)
	if err != nil {
		return err
	}

	t, err := h.svc.AdminThread(c.Context(), id)
	if err != nil {
		return err
	}

	detail, err := h.svc.Describe(c.Context(), t, chat.RoleBuyer)
	if err != nil {
		return err
	}

	msgs, err := h.svc.Recent(c.Context(), id)
	if err != nil {
		return err
	}

	web.AuditEvent(c.Request(), "admin.conversation_viewed", "conversation_id", id, "actor_id", u.AuthID())

	return inertia.Render(c, "Admin/Conversation", inertia.Props{
		"conversation": detail, "messages": msgs, "buyer_id": t.BuyerID,
	})
}
