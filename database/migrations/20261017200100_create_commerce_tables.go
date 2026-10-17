package migrations

import (
	"slices"

	"github.com/AlexGalhardo/vukem/schema"
)

// Commerce: addresses, cart, freight table, orders split per seller, the
// append-only ledger, payouts, provider webhook events and reviews.
// Money is bigint cents (BRL).
func m20261017200100CreateCommerceTables() schema.Migration {
	tables := []struct {
		name string
		fn   func(t *schema.Table)
	}{
		{"addresses", func(t *schema.Table) {
			t.ID()
			t.ForeignID("user_id").Constrained("users").CascadeOnDelete()
			t.String("recipient")
			t.String("cep", 8)
			t.String("street")
			t.String("number", 20)
			t.String("complement").Nullable()
			t.String("district")
			t.String("city")
			t.String("uf", 2)
			t.String("phone", 20).Nullable()
			t.Boolean("is_default").Default(false)
			t.Timestamps()
			t.Index("user_id")
		}},
		{"cart_items", func(t *schema.Table) {
			t.ID()
			t.ForeignID("user_id").Constrained("users").CascadeOnDelete()
			t.ForeignID("product_id").Constrained("products").CascadeOnDelete()
			t.Integer("quantity")
			t.Timestamps()
			t.UniqueIndex("user_id", "product_id")
		}},
		{"freight_rates", func(t *schema.Table) {
			t.ID()
			t.String("region", 2) // destination region: N, NE, CO, SE, S
			t.Integer("max_weight_grams")
			t.BigInteger("price_cents")
			t.Integer("days")
			t.UniqueIndex("region", "max_weight_grams")
		}},
		{"orders", func(t *schema.Table) {
			t.ID()
			t.UUID("public_id").Unique()
			t.ForeignID("buyer_id").Constrained("users").RestrictOnDelete()
			t.String("status").Default("pending") // pending | paid | partially_refunded | refunded | canceled | expired
			t.String("payment_method", 10)        // card | pix
			t.BigInteger("subtotal_cents")
			t.BigInteger("shipping_cents")
			t.BigInteger("fee_cents")
			t.BigInteger("total_cents")
			t.JSON("shipping_address")
			t.String("stripe_checkout_session_id").Nullable().Unique()
			t.String("stripe_payment_intent_id").Nullable()
			t.String("abacatepay_charge_id").Nullable().Unique()
			t.Text("pix_br_code").Nullable()
			t.Timestamp("pix_expires_at").Nullable()
			t.Timestamp("paid_at").Nullable()
			t.Timestamps()
			t.Index("buyer_id", "created_at")
			t.Index("status")
		}},
		{"seller_orders", func(t *schema.Table) {
			t.ID()
			t.ForeignID("order_id").Constrained("orders").CascadeOnDelete()
			t.ForeignID("shop_id").Constrained("shops").RestrictOnDelete()
			t.String("status").Default("pending") // pending | paid | shipped | delivered | refunded | canceled
			t.BigInteger("subtotal_cents")
			t.BigInteger("shipping_cents")
			t.BigInteger("fee_cents")
			t.BigInteger("payout_cents")
			t.String("stripe_transfer_id").Nullable()
			t.String("carrier").Nullable()
			t.String("tracking_code").Nullable()
			t.Timestamp("shipped_at").Nullable()
			t.Timestamp("delivered_at").Nullable()
			t.Timestamps()
			t.UniqueIndex("order_id", "shop_id")
			t.Index("shop_id", "status")
		}},
		{"order_items", func(t *schema.Table) {
			t.ID()
			t.ForeignID("seller_order_id").Constrained("seller_orders").CascadeOnDelete()
			t.ForeignID("product_id").Nullable().Constrained("products").NullOnDelete()
			t.String("title")
			t.BigInteger("price_cents")
			t.Integer("quantity")
			t.Integer("weight_grams")
			t.Timestamps()
			t.Index("seller_order_id")
		}},
		{"ledger_entries", func(t *schema.Table) {
			t.ID()
			// checkout.created, payment.succeeded, pix.received, seller.credit, transfer.created, refund, payout.paid...
			t.String("type")
			t.ForeignID("order_id").Nullable().Constrained("orders").RestrictOnDelete()
			t.ForeignID("seller_order_id").Nullable().Constrained("seller_orders").RestrictOnDelete()
			t.ForeignID("shop_id").Nullable().Constrained("shops").RestrictOnDelete()
			t.BigInteger("amount_cents") // signed
			t.String("provider", 20).Nullable()
			t.String("provider_ref").Nullable()
			// e.g. "transfer.created:<transfer id>": racing webhook deliveries cannot log twice.
			t.String("dedupe_key").Nullable().Unique()
			t.Timestamp("created_at")
			t.Index("shop_id", "created_at")
			t.Index("order_id")
		}},
		{"payouts", func(t *schema.Table) {
			t.ID()
			t.ForeignID("shop_id").Constrained("shops").RestrictOnDelete()
			t.BigInteger("amount_cents")
			t.String("status").Default("requested") // requested | paid | rejected
			t.String("pix_end_to_end_id").Nullable().Unique()
			t.ForeignID("paid_by").Nullable().Constrained("users").NullOnDelete()
			t.Timestamp("paid_at").Nullable()
			t.Text("rejection_reason").Nullable()
			t.Timestamps()
			t.Index("shop_id", "status")
		}},
		{"provider_events", func(t *schema.Table) {
			t.ID()
			t.String("provider", 20) // stripe | abacatepay
			t.String("event_id")
			t.String("type")
			t.String("payload_digest", 64)
			t.Timestamp("received_at")
			t.Timestamp("processed_at").Nullable()
			t.UniqueIndex("provider", "event_id")
		}},
		{"reviews", func(t *schema.Table) {
			t.ID()
			t.ForeignID("product_id").Constrained("products").CascadeOnDelete()
			t.ForeignID("buyer_id").Constrained("users").CascadeOnDelete()
			t.ForeignID("order_item_id").Unique().Constrained("order_items").CascadeOnDelete()
			t.Integer("rating")
			t.Text("body").Nullable()
			t.Timestamps()
			t.Index("product_id", "created_at")
		}},
	}

	return schema.Migration{
		ID: "20261017200100_create_commerce_tables",
		Up: func(s *schema.Schema) error {
			for _, tb := range tables {
				if err := s.Create(tb.name, tb.fn); err != nil {
					return err
				}
			}

			if s.Dialect() != "postgres" {
				return nil
			}

			return rawAll(s,
				`ALTER TABLE cart_items ADD CONSTRAINT cart_items_quantity_range CHECK (quantity BETWEEN 1 AND 99)`,
				`ALTER TABLE reviews ADD CONSTRAINT reviews_rating_range CHECK (rating BETWEEN 1 AND 5)`,
				`ALTER TABLE order_items ADD CONSTRAINT order_items_quantity_positive CHECK (quantity > 0)`,
				`ALTER TABLE payouts ADD CONSTRAINT payouts_amount_positive CHECK (amount_cents > 0)`,
				`CREATE UNIQUE INDEX addresses_one_default ON addresses (user_id) WHERE is_default`,
			)
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
