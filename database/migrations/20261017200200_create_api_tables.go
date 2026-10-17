package migrations

import (
	"slices"

	"github.com/AlexGalhardo/vukem/schema"
)

// API keys (mkt_<prefix>_<secret>, only the hash is stored), outgoing
// seller webhooks with their delivery log, and contact messages.
func m20261017200200CreateAPITables() schema.Migration {
	tables := []struct {
		name string
		fn   func(t *schema.Table)
	}{
		{"api_keys", func(t *schema.Table) {
			t.ID()
			t.ForeignID("user_id").Constrained("users").CascadeOnDelete()
			t.ForeignID("shop_id").Nullable().Constrained("shops").CascadeOnDelete()
			t.String("name")
			t.String("prefix", 16).Unique()
			t.String("secret_hash", 64)
			t.Text("scopes") // space-separated, e.g. "catalog:read cart:write"
			t.Timestamp("last_used_at").Nullable()
			t.Timestamp("expires_at").Nullable()
			t.Timestamp("revoked_at").Nullable()
			t.Timestamps()
			t.Index("user_id", "revoked_at")
		}},
		{"webhook_endpoints", func(t *schema.Table) {
			t.ID()
			t.ForeignID("shop_id").Constrained("shops").CascadeOnDelete()
			t.String("url", 2048)
			t.Text("secret") // orm.Encrypted ciphertext
			t.Text("events") // space-separated: order.paid order.refunded product.suspended
			t.Boolean("active").Default(true)
			t.Timestamps()
			t.Index("shop_id")
		}},
		{"webhook_deliveries", func(t *schema.Table) {
			t.ID()
			t.ForeignID("endpoint_id").Constrained("webhook_endpoints").CascadeOnDelete()
			t.UUID("event_id")
			t.String("event")
			t.String("status").Default("pending") // pending | delivered | failed
			t.Integer("attempts").Default(0)
			t.Timestamp("next_attempt_at").Nullable()
			t.Integer("response_code").Nullable()
			t.Text("last_error").Nullable()
			t.Timestamps()
			t.UniqueIndex("endpoint_id", "event_id")
			t.Index("status", "next_attempt_at")
		}},
		{"contact_messages", func(t *schema.Table) {
			t.ID()
			t.String("name")
			t.String("email")
			t.String("subject")
			t.Text("body")
			t.Timestamps()
		}},
	}

	return schema.Migration{
		ID: "20261017200200_create_api_tables",
		Up: func(s *schema.Schema) error {
			for _, tb := range tables {
				if err := s.Create(tb.name, tb.fn); err != nil {
					return err
				}
			}

			return nil
		},
		Down: func(s *schema.Schema) error {
			for _, tb := range slices.Backward(tables) {
				if err := s.DropIfExists(tb.name); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
