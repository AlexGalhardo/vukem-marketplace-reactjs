// Package maintenance holds the scheduled housekeeping tasks: expired
// tokens, old provider event digests and old webhook delivery rows.
package maintenance

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexGalhardo/vukem/database"
)

// Retention windows. Provider event digests outlive every provider's retry
// window (Stripe retries for 3 days), so dropping them after 90 days cannot
// let a redelivery through; the raw payloads expire in Mongo at 90 days too.
const (
	ProviderEventRetention   = 90 * 24 * time.Hour
	WebhookDeliveryRetention = 30 * 24 * time.Hour
)

// Pruner deletes rows that are no longer useful. Every method is
// idempotent, so overlapping or repeated runs are harmless.
type Pruner struct {
	DB *database.DB
	// ResetTTL is how long password reset links work (accounts.ResetTTL).
	ResetTTL time.Duration
	// Now is the clock (time.Now when nil).
	Now func() time.Time
}

func (p Pruner) now() time.Time {
	if p.Now == nil {
		return time.Now().UTC()
	}

	return p.Now().UTC()
}

// Tokens deletes expired password resets, remember-me tokens and API
// access tokens.
func (p Pruner) Tokens(ctx context.Context) error {
	now := p.now()
	ph := p.DB.Dialect().Placeholder(1)

	stmts := []struct {
		name, sql string
		arg       time.Time
	}{
		{"password_resets", "DELETE FROM password_resets WHERE created_at < " + ph, now.Add(-p.ResetTTL)},
		{"remember_tokens", "DELETE FROM remember_tokens WHERE expires_at < " + ph, now},
		{"access_tokens", "DELETE FROM access_tokens WHERE expires_at IS NOT NULL AND expires_at < " + ph, now},
	}

	for _, s := range stmts {
		if _, err := p.DB.ExecContext(ctx, s.sql, s.arg); err != nil {
			return fmt.Errorf("prune %s: %w", s.name, err)
		}
	}

	return nil
}

// ProviderEvents deletes processed provider event digests older than
// ProviderEventRetention. Unprocessed rows stay for investigation.
func (p Pruner) ProviderEvents(ctx context.Context) error {
	ph := p.DB.Dialect().Placeholder(1)

	_, err := p.DB.ExecContext(ctx,
		"DELETE FROM provider_events WHERE processed_at IS NOT NULL AND received_at < "+ph,
		p.now().Add(-ProviderEventRetention))
	if err != nil {
		return fmt.Errorf("prune provider_events: %w", err)
	}

	return nil
}

// WebhookDeliveries deletes finished delivery rows (delivered or failed)
// older than WebhookDeliveryRetention; pending ones are still retrying.
func (p Pruner) WebhookDeliveries(ctx context.Context) error {
	ph := p.DB.Dialect().Placeholder(1)

	_, err := p.DB.ExecContext(ctx,
		"DELETE FROM webhook_deliveries WHERE status <> 'pending' AND updated_at < "+ph,
		p.now().Add(-WebhookDeliveryRetention))
	if err != nil {
		return fmt.Errorf("prune webhook_deliveries: %w", err)
	}

	return nil
}
