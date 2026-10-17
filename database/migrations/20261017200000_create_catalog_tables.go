package migrations

import (
	"slices"

	"github.com/AlexGalhardo/vukem/schema"
)

// Catalog: categories, shops (one per user), products and their images.
// Products are public only when published and their shop is active and
// payable; the search column and trigram index are Postgres-only.
func m20261017200000CreateCatalogTables() schema.Migration {
	tables := []struct {
		name string
		fn   func(t *schema.Table)
	}{
		{"categories", func(t *schema.Table) {
			t.ID()
			t.String("slug").Unique()
			t.String("name_en")
			t.String("name_pt")
			t.ForeignID("parent_id").Nullable().Constrained("categories").NullOnDelete()
			t.Integer("position").Default(0)
			t.Timestamps()
		}},
		{"shops", func(t *schema.Table) {
			t.ID()
			t.ForeignID("owner_id").Unique().Constrained("users").RestrictOnDelete()
			t.String("slug").Unique()
			t.String("name")
			t.Text("description").Nullable()
			t.String("logo_key").Nullable()
			t.String("banner_key").Nullable()
			t.String("status").Default("active").Index() // active | suspended
			t.Text("suspension_reason").Nullable()
			t.String("stripe_account_id").Nullable().Unique()
			t.Boolean("stripe_ready").Default(false)
			t.Text("pix_key").Nullable() // orm.Encrypted ciphertext
			t.String("ship_from_uf", 2).Default("SP")
			t.Timestamps()
		}},
		{"products", func(t *schema.Table) {
			t.ID()
			t.ForeignID("shop_id").Constrained("shops").CascadeOnDelete()
			t.ForeignID("category_id").Nullable().Constrained("categories").NullOnDelete()
			t.String("slug").Unique()
			t.String("title")
			t.Text("description")
			t.BigInteger("price_cents")
			t.Integer("stock").Default(0)
			t.Integer("weight_grams").Default(0)
			t.String("status").Default("draft") // draft | published | archived | suspended
			t.Text("suspension_reason").Nullable()
			t.Integer("rating_avg_x100").Default(0) // 4.25 stars is stored as 425
			t.Integer("rating_count").Default(0)
			t.Timestamp("published_at").Nullable()
			t.Timestamps()
			t.Index("shop_id", "status")
			t.Index("status", "category_id")
		}},
		{"product_images", func(t *schema.Table) {
			t.ID()
			t.ForeignID("product_id").Constrained("products").CascadeOnDelete()
			t.String("key") // S3 key prefix; variants live under it (w320, w640, w1280)
			t.Integer("width")
			t.Integer("height")
			t.Integer("position").Default(0)
			t.String("alt").Default("")
			t.Timestamps()
			t.Index("product_id", "position")
		}},
	}

	return schema.Migration{
		ID: "20261017200000_create_catalog_tables",
		Up: func(s *schema.Schema) error {
			for _, tb := range tables {
				if err := s.Create(tb.name, tb.fn); err != nil {
					return err
				}
			}

			if s.Dialect() != "postgres" {
				return nil
			}

			// The 'simple' text search config: titles mix EN and PT-BR, so no stemming.
			return rawAll(s,
				`ALTER TABLE products ADD COLUMN search tsvector GENERATED ALWAYS AS (
					setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
					setweight(to_tsvector('simple', coalesce(description, '')), 'B')) STORED`,
				`CREATE INDEX products_search_idx ON products USING gin (search)`,
				`CREATE INDEX products_title_trgm_idx ON products USING gin (title gin_trgm_ops)`,
				`ALTER TABLE products ADD CONSTRAINT products_price_positive CHECK (price_cents > 0)`,
				`ALTER TABLE products ADD CONSTRAINT products_stock_non_negative CHECK (stock >= 0)`,
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

// rawAll runs Postgres-only statements in order.
func rawAll(s *schema.Schema, stmts ...string) error {
	for _, stmt := range stmts {
		if err := s.Raw(stmt); err != nil {
			return err
		}
	}

	return nil
}
