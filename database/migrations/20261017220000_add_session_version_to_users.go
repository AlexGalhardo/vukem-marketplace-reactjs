package migrations

import "github.com/AlexGalhardo/vukem/schema"

// session_version backs vukem's auth.SessionVersioner: bumping it (password
// change or reset, "log out other sessions") ends every browser session
// of the user on any session driver.
func m20261017220000AddSessionVersionToUsers() schema.Migration {
	return schema.Migration{
		ID: "20261017220000_add_session_version_to_users",
		Up: func(s *schema.Schema) error {
			return s.Table("users", func(t *schema.Table) {
				t.BigInteger("session_version").Default(0)
			})
		},
		Down: func(s *schema.Schema) error {
			return s.Table("users", func(t *schema.Table) { t.DropColumn("session_version") })
		},
	}
}
