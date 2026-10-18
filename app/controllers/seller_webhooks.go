package controllers

import (
	"errors"
	"time"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/requests"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/webhooks"
)

// webhookSecretFlash carries a new endpoint's secret to the next page
// render only (shown once).
const webhookSecretFlash = "webhook_secret"

// recentDeliveries is how many log rows the endpoint page shows.
const recentDeliveries = 50

// SellerWebhooks manages the current seller's webhook endpoints. Every
// lookup is scoped to the shop owned by the signed-in user.
type SellerWebhooks struct {
	Service *webhooks.Service
}

// EndpointView is an endpoint as the pages see it (never the secret).
type EndpointView struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// DeliveryView is one delivery log row.
type DeliveryView struct {
	EventID      string     `json:"event_id"`
	Event        string     `json:"event"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	ResponseCode *int64     `json:"response_code"`
	LastError    *string    `json:"last_error"`
	NextAttempt  *time.Time `json:"next_attempt_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func endpointView(e webhooks.Endpoint) EndpointView {
	return EndpointView{ID: e.ID, URL: e.URL, Events: e.EventList(), Active: e.Active, CreatedAt: e.CreatedAt}
}

// shop returns the signed-in seller's shop id; ok is false when the user
// has no shop yet.
func (h SellerWebhooks) shop(c *web.Ctx) (int64, bool, error) {
	u, err := auth.User[models.User](c, "web")
	if err != nil {
		return 0, false, err
	}

	id, err := h.Service.ShopOf(c.Context(), u.ID)
	if errors.Is(err, webhooks.ErrNoShop) {
		return 0, false, nil
	}

	return id, err == nil, err
}

// Index lists the endpoints and, right after creation, the new secret.
func (h SellerWebhooks) Index(c *web.Ctx) error {
	shopID, ok, err := h.shop(c)
	if err != nil {
		return err
	}

	views := []EndpointView{}

	if ok {
		list, err := h.Service.List(c.Context(), shopID)
		if err != nil {
			return err
		}

		for _, e := range list {
			views = append(views, endpointView(e))
		}
	}

	secret, _ := c.Session().Get(webhookSecretFlash).(string)

	return inertia.Render(c, "Seller/Webhooks", inertia.Props{
		"hasShop":   ok,
		"endpoints": views,
		"events":    webhooks.AllEvents(),
		"maxCount":  webhooks.MaxEndpoints,
		"secret":    nilIfEmpty(secret),
	})
}

// Store creates an endpoint and shows its secret once.
func (h SellerWebhooks) Store(c *web.Ctx) error {
	shopID, ok, err := h.shop(c)
	if err != nil {
		return err
	}

	if !ok {
		return orm.ErrNotFound
	}

	in, err := web.Parse(c, requests.WebhookEndpointSchema)
	if err != nil {
		return err
	}

	_, secret, err := h.Service.Create(c.Context(), shopID, in.URL, in.Events)
	if err != nil {
		return err
	}

	c.Session().Flash(webhookSecretFlash, secret)
	inertia.Flash(c, "status", i18n.T(c.Context(), "webhooks.created"))

	return c.Redirect("/seller/webhooks")
}

// Show renders one endpoint with its delivery log; another seller's
// endpoint is a 404.
func (h SellerWebhooks) Show(c *web.Ctx) error {
	shopID, ok, err := h.shop(c)
	if err != nil {
		return err
	}

	id, err := c.ParamInt("id")
	if !ok || err != nil {
		return orm.ErrNotFound
	}

	ep, err := h.Service.Find(c.Context(), shopID, id)
	if err != nil {
		return err
	}

	rows, err := h.Service.Recent(c.Context(), ep.ID, recentDeliveries)
	if err != nil {
		return err
	}

	deliveries := make([]DeliveryView, 0, len(rows))
	for i := range rows {
		d := &rows[i]
		v := DeliveryView{
			EventID:   d.EventID,
			Event:     d.Event,
			Status:    d.Status,
			Attempts:  d.Attempts,
			UpdatedAt: d.UpdatedAt,
		}
		if d.ResponseCode.Valid {
			v.ResponseCode = &d.ResponseCode.Int64
		}

		if d.LastError.Valid {
			v.LastError = &d.LastError.String
		}

		if d.NextAttemptAt.Valid {
			v.NextAttempt = &d.NextAttemptAt.V
		}

		deliveries = append(deliveries, v)
	}

	return inertia.Render(c, "Seller/WebhookDeliveries", inertia.Props{
		"endpoint":   endpointView(ep),
		"deliveries": deliveries,
	})
}

// Destroy deletes an endpoint of the seller's shop.
func (h SellerWebhooks) Destroy(c *web.Ctx) error {
	shopID, ok, err := h.shop(c)
	if err != nil {
		return err
	}

	id, err := c.ParamInt("id")
	if !ok || err != nil {
		return orm.ErrNotFound
	}

	if err := h.Service.Delete(c.Context(), shopID, id); err != nil {
		return err
	}

	inertia.Flash(c, "status", i18n.T(c.Context(), "webhooks.deleted"))

	return c.RedirectStatus(303, "/seller/webhooks")
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}

	return s
}
