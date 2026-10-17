package catalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexGalhardo/vukem/orm"
)

// maxSlugLen keeps slugs (and URLs) short; a random suffix may add 7 bytes.
const maxSlugLen = 72

// translit folds the Latin letters with diacritics used in Portuguese and
// Spanish titles to ASCII, so "Câmera Fotográfica" becomes
// "camera-fotografica". Anything else non-ASCII is dropped.
var translit = map[rune]string{
	'á': "a", 'à': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'í': "i", 'ì': "i", 'î': "i", 'ï': "i",
	'ó': "o", 'ò': "o", 'ô': "o", 'õ': "o", 'ö': "o",
	'ú': "u", 'ù': "u", 'û': "u", 'ü': "u",
	'ç': "c", 'ñ': "n", 'ý': "y", 'ÿ': "y", 'ß': "ss", 'æ': "ae", 'œ': "oe", 'ø': "o",
}

// Slugify turns a title into a URL slug: lower case ASCII letters and
// digits separated by single hyphens. It returns "item" for input without
// any usable character.
func Slugify(title string) string {
	var b strings.Builder

	hyphen := false

	for _, r := range strings.ToLower(title) {
		var part string

		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			part = string(r)
		case translit[r] != "":
			part = translit[r]
		default:
			hyphen = b.Len() > 0

			continue
		}

		if hyphen {
			b.WriteByte('-')

			hyphen = false
		}

		b.WriteString(part)

		if b.Len() >= maxSlugLen {
			break
		}
	}

	out := strings.TrimRight(b.String(), "-")
	if len(out) > maxSlugLen {
		out = strings.TrimRight(out[:maxSlugLen], "-")
	}

	if out == "" {
		return "item"
	}

	return out
}

// randomSuffix returns n random bytes as lower-case hex.
func randomSuffix(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (Go 1.24+)

	return hex.EncodeToString(b)
}

// slugAttempts bounds retries when a slug is taken concurrently.
const slugAttempts = 5

// insertWithSlug inserts a row whose slug derives from title: the plain
// slug first, then slug-<random> while the unique index rejects it. The
// unique constraint (not a pre-check) is the source of truth, so two
// sellers racing on the same title both succeed.
func insertWithSlug(ctx context.Context, title string, insert func(ctx context.Context, slug string) error) error {
	base := Slugify(title)
	slug := base

	for range slugAttempts {
		err := insert(ctx, slug)
		if !errors.Is(err, orm.ErrConflict) {
			return err
		}

		slug = base + "-" + randomSuffix(3)
	}

	return fmt.Errorf("catalog: no free slug for %q", base)
}

// NewAnonID returns a random id for a guest's view/search history.
func NewAnonID() string { return randomSuffix(16) }
