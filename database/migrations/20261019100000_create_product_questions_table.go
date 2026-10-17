package migrations

import "github.com/AlexGalhardo/vukem/schema"

// Product questions and their answers (app/qna). shop_id copies the
// product's shop so the seller inbox reads one index; products never move
// between shops, so the copy cannot drift. There is no derived counter.
func m20261019100000CreateProductQuestionsTable() schema.Migration {
	return schema.Migration{
		ID: "20261019100000_create_product_questions_table",
		Up: func(s *schema.Schema) error {
			return s.Create("product_questions", func(t *schema.Table) {
				t.ID()
				t.ForeignID("product_id").Constrained("products").CascadeOnDelete()
				t.ForeignID("shop_id").Constrained("shops").CascadeOnDelete()
				t.ForeignID("asker_id").Constrained("users").CascadeOnDelete()
				t.Text("body")
				t.Text("answer").Nullable()
				t.Timestamp("answered_at").Nullable()
				t.Timestamp("hidden_at").Nullable()
				t.ForeignID("hidden_by").Nullable().Constrained("users").NullOnDelete()
				t.Text("hidden_reason").Nullable()
				t.Timestamps()
				t.Index("product_id", "created_at")
				t.Index("shop_id", "answered_at", "created_at")
				t.Index("asker_id", "created_at")
			})
		},
		Down: func(s *schema.Schema) error { return s.DropIfExists("product_questions") },
	}
}
