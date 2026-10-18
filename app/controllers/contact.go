package controllers

import (
	"log/slog"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/alerts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/requests"
)

// Contact handles the public contact form.
type Contact struct {
	DB    *database.DB
	Queue func() (*queue.Queue, error)
	Log   *slog.Logger
}

// Show renders the form.
func (Contact) Show(c *web.Ctx) error { return inertia.Render(c, "Contact", nil) }

// Store saves the message and queues its forwarding to Telegram. The
// message is kept even when queueing fails (it is in contact_messages).
func (h Contact) Store(c *web.Ctx) error {
	in, err := web.Parse(c, requests.ContactSchema)
	if err != nil {
		return err
	}

	m := models.ContactMessage{Name: in.Name, Email: in.Email, Subject: in.Subject, Body: in.Body}
	if err := orm.Create(c.Context(), h.DB, &m); err != nil {
		return err
	}

	q, err := h.Queue()
	if err == nil {
		err = q.Dispatch(c.Context(), alerts.ForwardContact{MessageID: m.ID})
	}

	if err != nil {
		h.Log.WarnContext(c.Context(), "contact: forward not queued", "err", err, "message_id", m.ID)
	}

	inertia.Flash(c, "status", i18n.T(c.Context(), "contact.sent"))

	return c.Redirect("/contact")
}
