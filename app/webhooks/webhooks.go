// Package webhooks delivers signed events to the HTTPS endpoints sellers
// register (order.paid, order.refunded, product.suspended). Deliveries go
// through vukem's webhook.Sender: Standard Webhooks signatures, queued
// retries with backoff, and httpclient.Safe, which refuses loopback,
// private and metadata addresses at connect time (DNS rebinding and
// redirects included). Every attempt updates the webhook_deliveries row
// of its (endpoint, event) pair, which is the seller's delivery log.
//
// Other streams call Service.Dispatch:
//
//	wh, err := webhooks.New(app)
//	err = wh.Dispatch(ctx, shopID, webhooks.EventOrderPaid, map[string]any{"order_id": publicID})
package webhooks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/webhook"
)

// Events sellers can subscribe to.
const (
	EventOrderPaid        = "order.paid"
	EventOrderRefunded    = "order.refunded"
	EventProductSuspended = "product.suspended"
)

// AllEvents lists the events in display order.
func AllEvents() []string {
	return []string{EventOrderPaid, EventOrderRefunded, EventProductSuspended}
}

// Delivery statuses stored in webhook_deliveries.status.
const (
	StatusPending   = "pending"
	StatusDelivered = "delivered"
	StatusFailed    = "failed"
)

// MaxEndpoints bounds the endpoints of one shop (each event fans out to
// all of them).
const MaxEndpoints = 5

// retries is how many times a failed delivery is retried (9 attempts over
// about two hours with the backoff below).
const retries = 8

func backoff(attempt int) time.Duration { return queue.Exponential(30 * time.Second)(attempt) }

// Endpoint is a seller's webhook endpoint. The signing secret is
// encrypted at rest and never serialized.
type Endpoint struct {
	orm.Model

	ShopID int64                 `json:"-"`
	URL    string                `json:"url"`
	Secret orm.Encrypted[string] `json:"-"`
	Events string                `json:"events"`
	Active bool                  `json:"active"`
}

// Table maps Endpoint to webhook_endpoints.
func (Endpoint) Table() string { return "webhook_endpoints" }

// EventList returns the subscribed events.
func (e Endpoint) EventList() []string { return strings.Fields(e.Events) }

// Delivery is the state of one event for one endpoint.
type Delivery struct {
	orm.Model

	EndpointID    int64          `json:"-"`
	EventID       string         `json:"event_id"`
	Event         string         `json:"event"`
	Status        string         `json:"status"`
	Attempts      int            `json:"attempts"`
	NextAttemptAt orm.NullTime   `json:"next_attempt_at"`
	ResponseCode  sql.NullInt64  `json:"-"`
	LastError     sql.NullString `json:"-"`
}

// Table maps Delivery to webhook_deliveries.
func (Delivery) Table() string { return "webhook_deliveries" }

// Options tune a Service. The zero value is the production setting.
type Options struct {
	// InsecureAllowHTTP permits http:// endpoints (tests, local tools).
	InsecureAllowHTTP bool
	// InsecureAllow lists internal prefixes endpoints may reach (tests).
	InsecureAllow []netip.Prefix
	// Now is the clock (default time.Now).
	Now func() time.Time
}

// Service manages endpoints and dispatches events.
type Service struct {
	db     *database.DB
	queue  func() (*queue.Queue, error)
	sender *webhook.Sender
	opts   Options
}

// New returns the production service of app.
func New(app *vukem.App) (*Service, error) {
	return NewWithOptions(app, Options{})
}

// NewWithOptions returns a service with opts (tests).
func NewWithOptions(app *vukem.App, opts Options) (*Service, error) {
	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	if opts.Now == nil {
		opts.Now = time.Now
	}

	s := &Service{db: db, queue: app.Queue, opts: opts}

	s.sender, err = webhook.NewSender(webhook.SenderOptions{
		Endpoint:          s.endpoint,
		Log:               deliveryLog{s},
		OnGone:            s.disable,
		Logger:            app.Log,
		Retries:           retries,
		Backoff:           backoff,
		Now:               opts.Now,
		InsecureAllowHTTP: opts.InsecureAllowHTTP,
		InsecureAllow:     opts.InsecureAllow,
	})
	if err != nil {
		return nil, fmt.Errorf("webhooks: %w", err)
	}

	return s, nil
}

// Job is the queue registration of deliveries.
func (s *Service) Job() queue.Registration { return s.sender.Job() }

// Deliver makes one delivery attempt now (tests and manual redelivery).
func (s *Service) Deliver(ctx context.Context, d webhook.Delivery) error {
	return s.sender.Deliver(ctx, d)
}

// Envelope is the JSON body every endpoint receives.
type Envelope struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Data      any       `json:"data"`
}

// ErrUnknownEvent is returned by Dispatch for an event outside AllEvents.
var ErrUnknownEvent = errors.New("webhooks: unknown event")

// Dispatch queues event for every active endpoint of shopID subscribed to
// it. payload becomes the envelope's data; keep it free of buyer personal
// data (ids and amounts, not names or addresses). Delivery is at least
// once: receivers deduplicate by the webhook-id header (the event id).
//
// With the sync queue driver (development without Redis) the delivery runs
// inline and its error is returned; callers should log, not fail, on it.
func (s *Service) Dispatch(ctx context.Context, shopID int64, event string, payload any) error {
	if !slices.Contains(AllEvents(), event) {
		return fmt.Errorf("%w %q", ErrUnknownEvent, event)
	}

	endpoints, err := orm.QueryOf[Endpoint](s.db).Where("shop_id", shopID).Where("active", true).Get(ctx)
	if err != nil {
		return fmt.Errorf("webhooks: list endpoints of shop %d: %w", shopID, err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("webhooks: event id: %w", err)
	}

	body, err := json.Marshal(Envelope{ID: id.String(), Type: event, CreatedAt: s.opts.Now().UTC(), Data: payload})
	if err != nil {
		return fmt.Errorf("webhooks: encode %s: %w", event, err)
	}

	q, err := s.queue()
	if err != nil {
		return err
	}

	var errs []error

	for _, ep := range endpoints {
		if !slices.Contains(ep.EventList(), event) {
			continue
		}

		row := Delivery{EndpointID: ep.ID, EventID: id.String(), Event: event, Status: StatusPending}
		if err := orm.Create(ctx, s.db, &row); err != nil {
			errs = append(errs, fmt.Errorf("webhooks: log delivery: %w", err))
			continue
		}

		d := webhook.Delivery{ID: id.String(), Endpoint: strconv.FormatInt(ep.ID, 10), Payload: body}
		if err := s.sender.Send(ctx, q, d); err != nil {
			errs = append(errs, fmt.Errorf("webhooks: queue delivery to endpoint %d: %w", ep.ID, err))
		}
	}

	return errors.Join(errs...)
}

// endpoint is the Sender's lookup: the URL and secret are read at each
// attempt, so a deleted or disabled endpoint stops its pending retries.
func (s *Service) endpoint(ctx context.Context, id string) (webhook.Endpoint, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return webhook.Endpoint{}, webhook.ErrEndpointDisabled
	}

	ep, err := orm.QueryOf[Endpoint](s.db).Find(ctx, n)
	if errors.Is(err, orm.ErrNotFound) || (err == nil && !ep.Active) {
		return webhook.Endpoint{}, webhook.ErrEndpointDisabled
	}

	if err != nil {
		return webhook.Endpoint{}, err
	}

	return webhook.Endpoint{URL: ep.URL, Secret: ep.Secret.V}, nil
}

// disable turns off an endpoint that answered 410 Gone.
func (s *Service) disable(ctx context.Context, id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil //nolint:nilerr // not one of ours: nothing to disable
	}

	_, err = orm.QueryOf[Endpoint](s.db).Where("id", n).Update(ctx, orm.Set{"active": false})

	return err
}

// deliveryLog records Sender attempts on the webhook_deliveries row of
// the (endpoint, event) pair.
//
// workaround(vukem): webhook.SQLDeliveryLog wants its own one-row-per-
// attempt table (id, endpoint, event_id, status, duration_ms, error,
// attempted_at in Unix ms); our schema keeps one row per delivery with an
// attempt counter, so this adapter implements webhook.DeliveryLog instead.
type deliveryLog struct{ s *Service }

// Record implements webhook.DeliveryLog.
func (l deliveryLog) Record(ctx context.Context, a webhook.Attempt) error {
	id, err := strconv.ParseInt(a.Endpoint, 10, 64)
	if err != nil {
		return nil //nolint:nilerr // invalid ids never reach the table
	}

	row, err := orm.QueryOf[Delivery](l.s.db).Where("endpoint_id", id).Where("event_id", a.EventID).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return nil // delivered without a log row (manual redelivery)
	}

	if err != nil {
		return err
	}

	attempts := row.Attempts + 1
	set := orm.Set{"attempts": attempts, "next_attempt_at": nil, "last_error": nil, "response_code": nil}

	if a.Status != 0 {
		set["response_code"] = a.Status
	}

	switch {
	case a.Error == "" && a.Status >= 200 && a.Status <= 299:
		set["status"] = StatusDelivered
	case a.Status == 410, a.Error == webhook.AttemptInvalid, a.Status >= 300 && a.Status <= 399, attempts > retries:
		// Final: 410 Gone, a delivery that can never work, a redirect
		// (never followed) or retries exhausted.
		set["status"] = StatusFailed
		set["last_error"] = attemptError(a)
	default:
		set["status"] = StatusPending
		set["last_error"] = attemptError(a)
		set["next_attempt_at"] = a.At.Add(backoff(attempts)).UTC()
	}

	_, err = orm.QueryOf[Delivery](l.s.db).Where("id", row.ID).Update(ctx, set)

	return err
}

// attemptError is the seller-facing reason: a category from vukem (never
// the URL, secret or response body) or the HTTP status.
func attemptError(a webhook.Attempt) string {
	if a.Error == "" || a.Error == webhook.AttemptHTTPStatus {
		return "http_" + strconv.Itoa(a.Status)
	}

	return a.Error
}
