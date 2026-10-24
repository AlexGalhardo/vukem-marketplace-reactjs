package requests

import (
	"errors"
	"unicode"

	"github.com/AlexGalhardo/vukem/z"
)

// Password and email length bounds, in characters. The React forms mirror
// them (resources/js/lib/credentials.ts).
const (
	PasswordMin = 8
	PasswordMax = 32
	EmailMin    = 12
	EmailMax    = 48
)

// Password and email validation messages (i18n keys).
const (
	MsgPasswordLength = "account.errors.password_length"
	MsgPasswordWeak   = "account.errors.password_weak"
	MsgEmailLength    = "account.errors.email_length"
)

// PasswordSchema validates a new password: PasswordMin..PasswordMax
// characters with at least one lower case letter, one upper case letter,
// one digit and one special character (anything that is not a letter or
// digit).
func PasswordSchema() z.StringSchema {
	return z.String().
		Min(PasswordMin).
		Max(PasswordMax).
		Message("too_small", MsgPasswordLength).
		Message("too_big", MsgPasswordLength).
		Refine(func(s string) error {
			var lower, upper, digit, special bool

			for _, r := range s {
				switch {
				case unicode.IsLower(r):
					lower = true
				case unicode.IsUpper(r):
					upper = true
				case unicode.IsDigit(r):
					digit = true
				default:
					special = true
				}
			}

			if !lower || !upper || !digit || !special {
				return errors.New(MsgPasswordWeak)
			}

			return nil
		})
}

// EmailSchema validates the email of a new account: trimmed, a valid
// address of EmailMin..EmailMax characters.
func EmailSchema() z.StringSchema {
	return z.String().
		Trim().
		Min(EmailMin).
		Max(EmailMax).
		Message("too_small", MsgEmailLength).
		Message("too_big", MsgEmailLength).
		Email()
}
