package payments

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/vukem/config"
)

// DefaultAbacatePayBaseURL is the AbacatePay API host.
const DefaultAbacatePayBaseURL = "https://api.abacatepay.com"

// Config is the payments configuration, read from the environment.
// Card and PIX are each enabled only when fully configured; the app boots
// without them (the buttons are disabled), but in production a value that
// is set and invalid, or a half-configured provider, is a boot error.
type Config struct {
	FeeBPS int64
	// AppURL is the public base URL (redirects from Stripe).
	AppURL string

	StripeSecretKey      config.Secret
	StripePublishableKey string
	StripeWebhookSecrets []config.Secret
	// StripeAPIBase overrides api.stripe.com; honoured only in tests.
	StripeAPIBase string

	AbacatePayAPIKey        config.Secret
	AbacatePayWebhookSecret config.Secret
	// AbacatePayHMACKey checks X-Webhook-Signature (AbacatePay's public
	// webhook key by default).
	AbacatePayHMACKey config.Secret
	AbacatePayBaseURL string

	// Production enables production-only checks (live webhooks over https).
	Production bool
	// Problems lists configuration that disabled a provider outside
	// production; log them at boot.
	Problems []string
}

// CardEnabled reports whether card payments (Stripe) are configured.
func (c Config) CardEnabled() bool {
	return !c.StripeSecretKey.IsZero() && len(c.StripeWebhookSecrets) > 0
}

// PixEnabled reports whether PIX (AbacatePay) is configured.
func (c Config) PixEnabled() bool {
	return !c.AbacatePayAPIKey.IsZero() && !c.AbacatePayWebhookSecret.IsZero()
}

// ErrConfig wraps every configuration error.
var ErrConfig = errors.New("payments: invalid configuration")

// LoadConfig reads the configuration with get (app.Source.Get). test
// allows STRIPE_API_BASE (fake servers).
func LoadConfig(get func(string) string, production, test bool) (Config, error) {
	c := Config{
		FeeBPS:            DefaultFeeBPS,
		AppURL:            strings.TrimSuffix(strings.TrimSpace(get("APP_URL")), "/"),
		Production:        production,
		AbacatePayBaseURL: DefaultAbacatePayBaseURL,
		AbacatePayHMACKey: config.NewSecret(abacatePayPublicKey),
	}

	if raw := strings.TrimSpace(get("PLATFORM_FEE_BPS")); raw != "" {
		bps, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || ValidateFeeBPS(bps) != nil {
			return Config{}, fmt.Errorf("%w: PLATFORM_FEE_BPS must be an integer between 0 and 10000", ErrConfig)
		}

		c.FeeBPS = bps
	}

	var problems []string

	stripeProblems := c.loadStripe(get)
	if test {
		c.StripeAPIBase = strings.TrimSpace(get("STRIPE_API_BASE"))
	}

	pixProblems := c.loadPix(get)

	if len(stripeProblems) > 0 {
		c.StripeSecretKey, c.StripeWebhookSecrets, c.StripePublishableKey = config.Secret{}, nil, ""
		problems = append(problems, stripeProblems...)
	}

	if len(pixProblems) > 0 {
		c.AbacatePayAPIKey, c.AbacatePayWebhookSecret = config.Secret{}, config.Secret{}
		problems = append(problems, pixProblems...)
	}

	if production && len(problems) > 0 {
		return Config{}, fmt.Errorf("%w: %s", ErrConfig, strings.Join(problems, "; "))
	}

	c.Problems = problems

	return c, nil
}

// loadStripe reads the STRIPE_* keys and returns what is wrong with them.
// Nothing set is not a problem (card is just off).
func (c *Config) loadStripe(get func(string) string) []string {
	secret := strings.TrimSpace(get("STRIPE_SECRET_KEY"))
	publishable := strings.TrimSpace(get("STRIPE_PUBLISHABLE_KEY"))
	hooks := strings.TrimSpace(get("STRIPE_WEBHOOK_SECRETS"))

	if secret == "" && publishable == "" && hooks == "" {
		return nil
	}

	var problems []string

	if !strings.HasPrefix(secret, "sk_") && !strings.HasPrefix(secret, "rk_") {
		problems = append(problems, "STRIPE_SECRET_KEY must be set and start with sk_ or rk_")
	}

	if publishable != "" && !strings.HasPrefix(publishable, "pk_") {
		problems = append(problems, "STRIPE_PUBLISHABLE_KEY must start with pk_")
	}

	var secrets []config.Secret

	for s := range strings.SplitSeq(hooks, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}

		if !strings.HasPrefix(s, "whsec_") {
			problems = append(problems, "every STRIPE_WEBHOOK_SECRETS entry must start with whsec_")
			continue
		}

		secrets = append(secrets, config.NewSecret(s))
	}

	if len(secrets) == 0 {
		problems = append(problems, "STRIPE_WEBHOOK_SECRETS must list at least one whsec_ secret")
	}

	c.StripeSecretKey = config.NewSecret(secret)
	c.StripePublishableKey = publishable
	c.StripeWebhookSecrets = secrets

	return problems
}

// loadPix reads the ABACATEPAY_* keys and returns what is wrong with them.
func (c *Config) loadPix(get func(string) string) []string {
	key := strings.TrimSpace(get("ABACATEPAY_API_KEY"))
	hook := strings.TrimSpace(get("ABACATEPAY_WEBHOOK_SECRET"))
	base := strings.TrimSpace(get("ABACATEPAY_BASE_URL"))
	hmacKey := strings.TrimSpace(get("ABACATEPAY_WEBHOOK_PUBLIC_KEY"))

	if hmacKey != "" {
		c.AbacatePayHMACKey = config.NewSecret(hmacKey)
	}

	var problems []string

	if base != "" {
		u, err := url.Parse(base)
		if err != nil || u.Host == "" || (u.Scheme != "https" && (c.Production || u.Scheme != "http")) ||
			u.User != nil || u.RawQuery != "" {
			problems = append(problems, "ABACATEPAY_BASE_URL must be an https URL")
		} else {
			c.AbacatePayBaseURL = strings.TrimSuffix(base, "/")
		}
	}

	if key == "" && hook == "" {
		return problems
	}

	if key == "" {
		problems = append(problems, "ABACATEPAY_API_KEY must be set with ABACATEPAY_WEBHOOK_SECRET")
	}

	if hook == "" {
		problems = append(problems, "ABACATEPAY_WEBHOOK_SECRET must be set with ABACATEPAY_API_KEY")
	}

	c.AbacatePayAPIKey = config.NewSecret(key)
	c.AbacatePayWebhookSecret = config.NewSecret(hook)

	return problems
}

// abacatePayPublicKey is the HMAC key AbacatePay documents for checking
// X-Webhook-Signature (https://docs.abacatepay.com/pages/webhooks). It is
// public by design; override it with ABACATEPAY_WEBHOOK_PUBLIC_KEY if the
// provider rotates it.
const abacatePayPublicKey = "t9dXRhHHo3yDEj5pVDYz0frf7q6bMKyMRmxxCPIPp3RCplBfXRxqlC6ZpiWmOqj4L63qEaeUOtrCI8P0VMUgo6iIga2ri9" +
	"ogaHFs0WIIywSMg0q7RmBfybe1E5XJcfC4IW3alNqym0tXoAKkzvfEjZxV6bE0oG2zJrNNYmUCKZyV0KZ3JS8Votf9EAWWYdiDkMkpbMdPgg" +
	"fh1EqHlVkMiTady6jOR3hyzGEHrIz2Ret0xHKMbiqkr9HS1JhNHDX9"
