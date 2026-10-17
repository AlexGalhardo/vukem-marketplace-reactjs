package migrations

import "github.com/AlexGalhardo/vukem/schema"

// Feature flags set by admins (app/flags): a row only for flags changed
// from their default, plus an append-only log of every change.
func m20261018100000CreateFeatureFlagsTables() schema.Migration {
	return schema.Migration{
		ID: "20261018100000_create_feature_flags_tables",
		Up: func(s *schema.Schema) error {
			if err := s.Create("feature_flags", func(t *schema.Table) {
				t.ID()
				t.String("name", 64).Unique()
				t.Boolean("enabled")
				t.Timestamps()
			}); err != nil {
				return err
			}

			return s.Create("feature_flag_changes", func(t *schema.Table) {
				t.ID()
				t.String("flag", 64)
				t.Boolean("old_enabled")
				t.Boolean("new_enabled")
				t.ForeignID("user_id").Nullable().Constrained("users").NullOnDelete()
				// A copy of the admin's email: the log outlives the account.
				t.String("user_email")
				t.Timestamp("created_at")
				t.Index("created_at")
			})
		},
		Down: func(s *schema.Schema) error {
			if err := s.DropIfExists("feature_flag_changes"); err != nil {
				return err
			}

			return s.DropIfExists("feature_flags")
		},
	}
}
