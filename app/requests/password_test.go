package requests_test

import (
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/requests"
)

func TestPasswordSchema(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, in string
		ok       bool
	}{
		{"valid", "Abcdef1!", true},
		{"max length", "Aa1!" + strings.Repeat("x", 28), true},
		{"too short", "Ab1!xyz", false},
		{"too long", "Aa1!" + strings.Repeat("x", 29), false},
		{"no lower", "ABCDEF1!", false},
		{"no upper", "abcdef1!", false},
		{"no digit", "Abcdefg!", false},
		{"no special", "Abcdefg1", false},
	} {
		_, err := requests.PasswordSchema().Parse(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("%s: Parse(%q) error = %v, want ok=%v", tc.name, tc.in, err, tc.ok)
		}
	}
}

func TestEmailSchema(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in string
		ok bool
	}{
		{"ana@example.com", true},
		{"a@b.co", false},
		{strings.Repeat("a", 40) + "@example.com", false},
		{"not-an-email-at-all", false},
	} {
		_, err := requests.EmailSchema().Parse(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("Parse(%q) error = %v, want ok=%v", tc.in, err, tc.ok)
		}
	}
}
