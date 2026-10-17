package migrations

import "github.com/AlexGalhardo/vukem/schema"

// role is "user" or "admin"; admins are only created by the
// users:make-admin command, never through the web or the API.
func m20261017150000AddRoleToUsers() schema.Migration {
	return schema.Migration{
		ID: "20261017150000_add_role_to_users",
		Up: func(s *schema.Schema) error {
			return s.Table("users", func(t *schema.Table) {
				t.String("role").Default("user").Index()
			})
		},
		Down: func(s *schema.Schema) error {
			return s.Table("users", func(t *schema.Table) { t.DropColumn("role") })
		},
	}
}
