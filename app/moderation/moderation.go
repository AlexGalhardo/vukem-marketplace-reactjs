// Package moderation holds the content rules shared by user-written text
// (reviews, seller replies): no way to take the buyer off the platform.
package moderation

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	urlRE    = regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+`)
	domainRE = regexp.MustCompile(
		`(?i)\b[a-z0-9][a-z0-9-]*\.(?:com|net|org|br|io|me|app|dev|info|biz|co|link|shop|store|site|online|xyz|ly|gl)\b`,
	)
	emailRE = regexp.MustCompile(
		`(?i)[a-z0-9._%+-]+\s*(?:@|\(at\)|\[at\])\s*[a-z0-9-]+(?:\s*(?:\.|\(dot\)|\[dot\])\s*[a-z]{2,})+`,
	)
	handleRE = regexp.MustCompile(`(?i)\b(?:whats\s*app|whatsapp|zap|telegram|wa\.me|t\.me|instagram|insta)\b`)
)

// minPhoneDigits is the shortest digit run treated as a phone number: a
// Brazilian landline without area code has 8 digits.
const minPhoneDigits = 8

// ContainsContact reports whether text carries a link, a domain, an email
// address, a messaging handle or a phone number (eight or more digits,
// separators such as spaces, dots, dashes and parentheses ignored).
func ContainsContact(text string) bool {
	if urlRE.MatchString(text) || domainRE.MatchString(text) || emailRE.MatchString(text) ||
		handleRE.MatchString(text) {
		return true
	}

	return hasPhone(text)
}

// hasPhone looks for a run of minPhoneDigits digits where only phone
// separators may sit between them, so "1.000 reais em 3 vezes" passes and
// "(11) 9 8765-4321" does not.
func hasPhone(text string) bool {
	digits := 0

	for _, r := range text {
		switch {
		case unicode.IsDigit(r):
			digits++
			if digits >= minPhoneDigits {
				return true
			}
		case strings.ContainsRune(" .-()+", r):
			// separator: keep counting
		default:
			digits = 0
		}
	}

	return false
}
