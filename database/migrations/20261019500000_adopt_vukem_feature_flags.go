package migrations

import (
	vflags "github.com/AlexGalhardo/vukem/flags"
	"github.com/AlexGalhardo/vukem/schema"
)

// The app's feature_flags(name, enabled) table becomes vukem's
// feature_flags(name, scope, value, updated_at) (vukem flags.Migration),
// keeping every stored value. The admin TOTP flag is locked in code, so a
// stored row for it is dropped. feature_flag_changes (the audit log) stays.
func m20261019500000AdoptVukemFeatureFlags() schema.Migration {
	const legacy = "feature_flags_v1"

	vukemTable := vflags.Migration("20261019500000_adopt_vukem_feature_flags")

	return schema.Migration{
		ID: vukemTable.ID,
		Up: func(s *schema.Schema) error {
			if err := s.Rename("feature_flags", legacy); err != nil {
				return err
			}

			if err := vukemTable.Up(s); err != nil {
				return err
			}

			if err := s.Raw(`INSERT INTO feature_flags (name, scope, value, updated_at)
				SELECT name, '', CASE WHEN enabled THEN 'true' ELSE 'false' END, updated_at
				FROM ` + legacy + ` WHERE name <> 'admin_two_factor'`); err != nil {
				return err
			}

			return s.Drop(legacy)
		},
		Down: func(s *schema.Schema) error {
			if err := s.Create(legacy, func(t *schema.Table) {
				t.ID()
				t.String("name", 64).Unique()
				t.Boolean("enabled")
				t.Timestamps()
			}); err != nil {
				return err
			}

			if err := s.Raw(`INSERT INTO ` + legacy + ` (name, enabled, created_at, updated_at)
				SELECT name, value = 'true', updated_at, updated_at
				FROM feature_flags WHERE scope = ''`); err != nil {
				return err
			}

			if err := vukemTable.Down(s); err != nil {
				return err
			}

			return s.Rename(legacy, "feature_flags")
		},
	}
}
