// Package sqlbind rewrites '?' placeholders for the connection's dialect,
// so hand-written queries run on Postgres ($1) and on the SQLite test
// database (?) alike.
package sqlbind

import (
	"strings"

	"github.com/AlexGalhardo/vukem/database"
)

// Rebind replaces every '?' in query with d's placeholder. Queries passed
// here must not contain '?' inside string literals.
func Rebind(d database.Dialect, query string) string {
	if d.Placeholder(1) == "?" {
		return query
	}

	var (
		b strings.Builder
		n int
	)

	b.Grow(len(query) + 8)

	for _, r := range query {
		if r == '?' {
			n++
			b.WriteString(d.Placeholder(n))

			continue
		}

		b.WriteRune(r)
	}

	return b.String()
}
