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
		m20261017150000AddRoleToUsers(),
		m20261017200000CreateCatalogTables(),
		m20261017200100CreateCommerceTables(),
		m20261017200200CreateAPITables(),
		m20261017220000AddSessionVersionToUsers(),
		m20261018100000CreateFeatureFlagsTables(),
		m20261019100000CreateProductQuestionsTable(),
		m20261019200000ExtendReviews(),
		m20261019400000CreateChatTables(),
	}
}
