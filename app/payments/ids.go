package payments

import (
	"regexp"

	"github.com/AlexGalhardo/vukem/uuid"
)

// NewPublicID returns a UUIDv7 (RFC 9562): time-ordered, and it never
// reveals how many orders exist.
func NewPublicID() string { return uuid.NewV7().String() }

var publicIDRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ValidPublicID reports whether s is a lower-case UUID, checked before it
// reaches a uuid column (Postgres rejects malformed input with an error).
func ValidPublicID(s string) bool { return publicIDRe.MatchString(s) }
