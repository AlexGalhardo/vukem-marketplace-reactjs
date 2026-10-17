package requests

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AlexGalhardo/vukem/z"
)

// Name length bounds, in characters (runes) after NormalizeName. The
// React forms use the same numbers (resources/js/lib/name.ts).
const (
	NameMin = 4
	NameMax = 24
)

// MsgNameLength is the message for a name outside NameMin..NameMax.
const MsgNameLength = "account.errors.name_length"

// NormalizeName trims s and collapses every run of white space inside it
// (any Unicode space, tabs and newlines included) to one space.
func NormalizeName(s string) string { return strings.Join(strings.Fields(s), " ") }

// CapitalizeName upper-cases the first character of s ("álvaro" →
// "Álvaro"); the rest is kept as typed. A first character without an
// upper case (a digit, an emoji, "ß") is left alone.
func CapitalizeName(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 || r == utf8.RuneError {
		return s
	}

	return string(unicode.ToUpper(r)) + s[n:]
}

// NameSchema validates a person's name: normalized, NameMin..NameMax
// runes, first letter capitalized.
func NameSchema() z.StringSchema {
	return z.String().
		Preprocess(NormalizeName).
		Min(NameMin).
		Max(NameMax).
		Message("too_small", MsgNameLength).
		Message("too_big", MsgNameLength).
		Transform(CapitalizeName)
}
