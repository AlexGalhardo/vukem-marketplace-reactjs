// Package migrations holds the schema migrations.
package migrations

import "github.com/AlexGalhardo/vukem/schema"

// All lists the migrations in order; vukem make:migration appends here.
func All() []schema.Migration {
	return []schema.Migration{
		m20261017100000EnablePostgresExtensions(),
		m20261017131452CreateUsersTable(),
		m20261017131453CreateAuthTables(),
		m20261017131454CreateSocialAccountsTable(),
		m20261017131455AddTwoFactorColumns(),
	}
}
