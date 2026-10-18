package routes

import (
	"fmt"
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/sse"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers/messages"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/platform"
)

// Chat registers the buyer ↔ seller chat behind the chat flag: inboxes,
// conversation pages and their JSON endpoints (per-user rate limits), the
// realtime transports (/broadcast/ws, /broadcast/sse fallback) and the
// admin's read-only view. CHAT_NOTIFY_AFTER (a Go duration, default 10m)
// is how long a recipient must be away before unread messages are
// emailed.
func Chat(deps *platform.Deps) func(r *router.Router, app *vukem.App) error {
	return func(r *router.Router, app *vukem.App) error {
		_, err := ChatHandlers(r, app, deps)
		return err
	}
}

// ChatHandlers registers the chat routes and returns their handlers
// (tests reach the service and broadcast server through them).
func ChatHandlers(r *router.Router, app *vukem.App, deps *platform.Deps) (*messages.Handlers, error) {
	acc, err := accounts.New(app)
	if err != nil {
		return nil, err
	}

	notifyAfter := time.Duration(0)

	if v, ok := app.Source.Lookup("CHAT_NOTIFY_AFTER"); ok && v != "" {
		notifyAfter, err = time.ParseDuration(v)
		if err != nil || notifyAfter <= 0 {
			return nil, fmt.Errorf("routes: CHAT_NOTIFY_AFTER %q must be a positive duration such as 10m", v)
		}
	}

	var broker sse.Broker
	if deps.Redis != nil {
		broker = deps.Redis
	}

	h, err := messages.FromApp(app, acc, messages.Options{
		Broker: broker, Prefix: deps.RedisPrefix() + "chat:", NotifyAfter: notifyAfter,
	})
	if err != nil {
		return nil, err
	}

	limiters, err := app.RateLimiters()
	if err != nil {
		return nil, err
	}

	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	limit := func(name string, n int, window time.Duration) router.Middleware {
		return ratelimit.Middleware(limiters(name, n, window), messages.ThrottleKey)
	}

	on := h.RequireChat
	user := []router.Middleware{on, acc.RequireVerified}
	with := func(mw ...router.Middleware) []router.Middleware {
		return append(append([]router.Middleware{}, user...), mw...)
	}
	L := messages.Localized

	r.Use(h.Share)

	r.Get("/messages", h.Index).Name("messages").Use(user...)
	r.Post("/messages", L(h.Start)).Name("messages.store").Use(with(limit("chat-start", 10, 10*time.Minute))...)
	r.Get("/messages/{id}", h.Show).Name("messages.show").Use(user...)
	r.Post("/messages/{id}", L(h.Send)).Name("messages.send").Use(with(limit("chat-send", 30, time.Minute))...)
	r.Get("/messages/{id}/since", h.Since).Name("messages.since").Use(with(limit("chat-since", 60, time.Minute))...)
	r.Post("/messages/{id}/read", h.Read).Name("messages.read").Use(with(limit("chat-read", 120, time.Minute))...)
	r.Post("/messages/{id}/typing", h.Typing).Name("messages.typing").
		Use(with(limit("chat-typing", 30, time.Minute))...)
	r.Get("/seller/messages", h.SellerIndex).Name("seller.messages").Use(with(requireShop(acc, db))...)

	// Reconnects back off on the client; this stops loops and floods.
	socket := limit("chat-socket", 30, time.Minute)
	r.Get("/broadcast/ws", h.Socket).Name("broadcast.ws").Use(with(socket)...)
	r.Get("/broadcast/sse", h.Stream).Name("broadcast.sse").Use(with(socket)...)

	r.Group("/admin/messages", func(g *router.Group) {
		g.Use(on, acc.RequireAdmin, acc.RequireVerified)
		g.Get("", h.AdminIndex).Name("admin.messages")
		g.Get("/{id}", h.AdminShow).Name("admin.messages.show")
	})

	return h, nil
}
