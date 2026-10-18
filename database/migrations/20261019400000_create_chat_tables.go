package migrations

import "github.com/AlexGalhardo/vukem/schema"

// Buyer ↔ seller chat (app/chat). One conversation per buyer, shop and
// optional product; the seller side is the shop's owner (shops never
// change owner). Read receipts are the last message id each side has
// read; last_message_id/last_message_at are a copy of the newest message
// kept in the same transaction as its insert, so the inbox reads one row.
func m20261019400000CreateChatTables() schema.Migration {
	return schema.Migration{
		ID: "20261019400000_create_chat_tables",
		Up: func(s *schema.Schema) error {
			err := s.Create("conversations", func(t *schema.Table) {
				t.ID()
				t.ForeignID("buyer_id").Constrained("users").CascadeOnDelete()
				t.ForeignID("shop_id").Constrained("shops").CascadeOnDelete()
				t.ForeignID("product_id").Nullable().Constrained("products").NullOnDelete()
				t.BigInteger("last_message_id").Default(0)
				t.Timestamp("last_message_at").Nullable()
				t.BigInteger("buyer_last_read_id").Default(0)
				t.BigInteger("seller_last_read_id").Default(0)
				t.Timestamps()
				t.Index("buyer_id", "last_message_at")
				t.Index("shop_id", "last_message_at")
			})
			if err != nil {
				return err
			}

			// A NULL product would make a plain unique index accept
			// duplicates; both dialects index the expression.
			err = s.Raw(`CREATE UNIQUE INDEX conversations_thread_unique
				ON conversations (buyer_id, shop_id, COALESCE(product_id, 0))`)
			if err != nil {
				return err
			}

			return s.Create("chat_messages", func(t *schema.Table) {
				t.ID()
				t.ForeignID("conversation_id").Constrained("conversations").CascadeOnDelete()
				t.ForeignID("sender_id").Constrained("users").CascadeOnDelete()
				t.Text("body")
				t.Timestamps()
				t.Index("conversation_id", "id")
			})
		},
		Down: func(s *schema.Schema) error {
			if err := s.DropIfExists("chat_messages"); err != nil {
				return err
			}

			return s.DropIfExists("conversations")
		},
	}
}
