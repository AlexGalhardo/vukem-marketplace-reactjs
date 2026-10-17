package qna

import "regexp"

// Contact patterns that move a conversation off the platform. The same
// rules run in the browser (resources/js/lib/qna.ts); keep them in sync.
var (
	contactURL = regexp.MustCompile(`(?i)\b(?:https?|ftp)://|\bwww\d{0,3}\.|` +
		// A bare domain: no spaces around the dot, or "produto. Com certeza"
		// (Portuguese "com" = "with") would match.
		`\b[a-z0-9][a-z0-9-]*\.(?:com|net|org|br|io|me|ly|gl|app|dev|shop|store|link|site|online|xyz|info|biz|co|tv|gg)\b`)
	contactEmail = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)*\.[a-z]{2,}|` +
		// Social handles (@shop_name).
		`(?:^|[\s(:])@[a-z0-9_][a-z0-9_.]{2,}`)
	contactObfuscated = regexp.MustCompile(`(?i)\barroba\b|[\[(]\s*(?:at|dot|ponto)\s*[\])]|` +
		`\b(?:ponto|dot)\s*(?:com|net|org|br)\b|\bpontocom\b|\b(?:gmail|hotmail)\b`)
	// A run of digits joined by phone separators; its digits are counted.
	digitRun = regexp.MustCompile(`\d[\d\s.()+/-]*\d`)
	// An 8-digit landline written with a separator (3333-4444). A CEP
	// (01310-100) has a 5-3 shape and stays allowed.
	landline = regexp.MustCompile(`\d{4}[\s.-]\d{4}`)
)

// minPhoneDigits is the shortest digit run treated as a phone number
// without a landline shape: a mobile number without its area code.
const minPhoneDigits = 9

// ContainsContact reports whether s carries a link, an email, a social
// handle or a phone number, including common obfuscations ("arroba",
// "ponto com", digits split by spaces, dots or dashes).
func ContainsContact(s string) bool {
	if contactURL.MatchString(s) || contactEmail.MatchString(s) || contactObfuscated.MatchString(s) {
		return true
	}

	for _, run := range digitRun.FindAllString(s, -1) {
		digits := 0

		for _, r := range run {
			if r >= '0' && r <= '9' {
				digits++
			}
		}

		if digits >= minPhoneDigits || (digits == 8 && landline.MatchString(run)) {
			return true
		}
	}

	return false
}
