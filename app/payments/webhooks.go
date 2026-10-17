package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	vstripe "github.com/AlexGalhardo/vukem/contrib/stripe"
)

// Webhook paths. They are served ahead of the session and CSRF middleware:
// providers are servers, not browsers; signatures authenticate them.
const (
	StripeWebhookPath     = "/webhooks/stripe"
	AbacatePayWebhookPath = "/webhooks/abacatepay"
)

const maxWebhookBody = 1 << 20

// WebhookMiddleware answers the two provider webhooks and passes every
// other request on. Mount it with router.UseServer.
func (s *Service) WebhookMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case StripeWebhookPath:
			s.stripeWebhook(w, r)
		case AbacatePayWebhookPath:
			s.pixWebhook(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return nil, false
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}

	return body, true
}

// stripeWebhook verifies Stripe-Signature against each configured secret
// (platform and Connect endpoints have their own), dedupes the event id in
// provider_events and processes it; failures answer 500 so Stripe retries.
func (s *Service) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, ok := readBody(w, r)
	if !ok {
		return
	}

	if !s.Config.CardEnabled() || s.Stripe == nil {
		http.Error(w, "card payments are not configured", http.StatusServiceUnavailable)
		return
	}

	header := r.Header.Get("Stripe-Signature")

	var (
		ev  vstripe.Event
		err = vstripe.ErrSignature
	)

	for _, secret := range s.Config.StripeWebhookSecrets {
		ev, err = vstripe.ConstructEvent(payload, header, secret.Reveal(), vstripe.DefaultTolerance, s.now())
		if err == nil {
			break
		}
	}

	if err != nil {
		s.log().WarnContext(r.Context(), "stripe webhook rejected", "event", "payments.webhook_rejected",
			"provider", ProviderStripe)
		http.Error(w, "invalid signature", http.StatusBadRequest)

		return
	}

	s.process(w, r, ProviderStripe, ev.ID, ev.Type, payload, func(ctx context.Context) error {
		return s.HandleStripeEvent(ctx, ev)
	})
}

// pixWebhook checks the webhookSecret query parameter (it proves the
// origin) and the X-Webhook-Signature HMAC-SHA256 of the raw body (it
// proves integrity), both in constant time, then dedupes and processes.
func (s *Service) pixWebhook(w http.ResponseWriter, r *http.Request) {
	payload, ok := readBody(w, r)
	if !ok {
		return
	}

	if !s.Config.PixEnabled() || s.Pix == nil {
		http.Error(w, "PIX is not configured", http.StatusServiceUnavailable)
		return
	}

	given := r.URL.Query().Get("webhookSecret")
	want := s.Config.AbacatePayWebhookSecret.Reveal()

	if subtle.ConstantTimeCompare([]byte(given), []byte(want)) != 1 ||
		!ValidAbacatePaySignature(payload, r.Header.Get("X-Webhook-Signature"), s.Config.AbacatePayHMACKey.Reveal()) {
		s.log().WarnContext(r.Context(), "abacatepay webhook rejected", "event", "payments.webhook_rejected",
			"provider", ProviderAbacatePay)
		http.Error(w, "unauthorized", http.StatusUnauthorized)

		return
	}

	var ev PixEvent
	if err := json.Unmarshal(payload, &ev); err != nil || ev.ID == "" || ev.Event == "" {
		http.Error(w, "not an event", http.StatusBadRequest)
		return
	}

	s.process(w, r, ProviderAbacatePay, ev.ID, ev.Event, payload, func(ctx context.Context) error {
		return s.HandlePixEvent(ctx, ev)
	})
}

// ValidAbacatePaySignature checks base64(HMAC-SHA256(key, body)).
func ValidAbacatePaySignature(body []byte, signature, key string) bool {
	if signature == "" || key == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)

	return hmac.Equal([]byte(base64.StdEncoding.EncodeToString(mac.Sum(nil))), []byte(signature))
}

// SignAbacatePayForTest signs a body like AbacatePay does.
func SignAbacatePayForTest(body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)

	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) process(
	w http.ResponseWriter,
	r *http.Request,
	provider, eventID, eventType string,
	payload []byte,
	handle func(context.Context) error,
) {
	// The provider's request may be cut short; finish what was started.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
	defer cancel()

	events := s.Events()

	if err := events.Claim(ctx, provider, eventID, eventType, payload); err != nil {
		if errors.Is(err, ErrEventProcessed) {
			w.WriteHeader(http.StatusOK) // duplicate delivery
			return
		}

		s.log().ErrorContext(ctx, "webhook claim failed", "provider", provider, "err", err)
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)

		return
	}

	if err := handle(ctx); err != nil {
		s.log().ErrorContext(ctx, "webhook processing failed", "provider", provider, "type", eventType,
			"id", eventID, "err", err)
		http.Error(w, "processing failed", http.StatusInternalServerError)

		return
	}

	if err := events.Done(ctx, provider, eventID); err != nil {
		s.log().ErrorContext(ctx, "webhook done failed", "provider", provider, "err", err)
	}

	w.WriteHeader(http.StatusOK)
}
