package migrations

import "github.com/AlexGalhardo/vukem/schema"

// Reviews (app/reviews): an optional title and "would recommend", the
// shop's single reply, admin moderation (a hidden review leaves the
// product's average) and the "helpful" votes, one per user and review.
func m20261019200000ExtendReviews() schema.Migration {
	return schema.Migration{
		ID: "20261019200000_extend_reviews",
		Up: func(s *schema.Schema) error {
			if err := s.Table("reviews", func(t *schema.Table) {
				t.String("title", 80).Nullable()
				t.Boolean("recommend").Nullable()
				t.Text("seller_reply").Nullable()
				t.Timestamp("seller_replied_at").Nullable()
				t.Timestamp("hidden_at").Nullable()
				t.ForeignID("hidden_by").Nullable().Constrained("users").NullOnDelete()
				t.Text("hidden_reason").Nullable()
				t.Integer("helpful_count").Default(0)
				t.Index("product_id", "helpful_count")
			}); err != nil {
				return err
			}

			if err := s.Create("review_votes", func(t *schema.Table) {
				t.ID()
				t.ForeignID("review_id").Constrained("reviews").CascadeOnDelete()
				t.ForeignID("user_id").Constrained("users").CascadeOnDelete()
				t.Timestamp("created_at")
				t.UniqueIndex("review_id", "user_id")
			}); err != nil {
				return err
			}

			if s.Dialect() != "postgres" {
				return nil
			}

			return rawAll(
				s,
				`ALTER TABLE reviews ADD CONSTRAINT reviews_helpful_non_negative CHECK (helpful_count >= 0)`,
				`ALTER TABLE reviews ADD CONSTRAINT reviews_hidden_reason CHECK (hidden_at IS NULL OR hidden_reason IS NOT NULL)`,
			)
		},
		Down: func(s *schema.Schema) error {
			if err := s.DropIfExists("review_votes"); err != nil {
				return err
			}

			if s.Dialect() == "postgres" {
				if err := rawAll(s,
					`ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_helpful_non_negative`,
					`ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_hidden_reason`,
				); err != nil {
					return err
				}
			}

			return s.Table("reviews", func(t *schema.Table) {
				t.DropIndex("reviews_product_id_helpful_count_index")
				t.DropColumn("title", "recommend", "seller_reply", "seller_replied_at", "hidden_at", "hidden_by",
					"hidden_reason", "helpful_count")
			})
		},
	}
}
