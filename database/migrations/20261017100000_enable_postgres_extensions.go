package migrations

import "github.com/AlexGalhardo/vukem/schema"

// citext backs case-insensitive emails and slugs; pg_trgm backs the
// catalog search (trigram indexes on product titles). Postgres-only: the
// in-memory SQLite used by HTTP-level unit tests skips it.
func m20261017100000EnablePostgresExtensions() schema.Migration {
	return schema.Migration{
		ID: "20261017100000_enable_postgres_extensions",
		Up: func(s *schema.Schema) error {
			if s.Dialect() != "postgres" {
				return nil
			}

			if err := s.Raw(`CREATE EXTENSION IF NOT EXISTS citext`); err != nil {
				return err
			}

			return s.Raw(`CREATE EXTENSION IF NOT EXISTS pg_trgm`)
		},
		Down: func(s *schema.Schema) error {
			if s.Dialect() != "postgres" {
				return nil
			}

			if err := s.Raw(`DROP EXTENSION IF EXISTS pg_trgm`); err != nil {
				return err
			}

			return s.Raw(`DROP EXTENSION IF EXISTS citext`)
		},
	}
}
