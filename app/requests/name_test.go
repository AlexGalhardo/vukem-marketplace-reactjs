package requests_test

import (
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/requests"
)

func TestNameSchema(t *testing.T) {
	t.Parallel()

	const invalid = ""

	cases := []struct {
		name, in, want string
	}{
		{"3 runes", "Ana", invalid},
		{"4 runes", "anna", "Anna"},
		{"24 runes", strings.Repeat("b", 24), "B" + strings.Repeat("b", 23)},
		{"25 runes", strings.Repeat("b", 25), invalid},
		{"accented first letter", "álvaro", "Álvaro"},
		{"cedilla and tilde", "joão da conceição", "João da conceição"},
		{"4 accented runes are 8 bytes", "éééé", "Éééé"},
		{"24 accented runes", strings.Repeat("é", 24), "É" + strings.Repeat("é", 23)},
		{"25 accented runes", strings.Repeat("é", 25), invalid},
		{"3 accented runes", "ééé", invalid},
		{"4 emoji", "😀😀😀😀", "😀😀😀😀"},
		{"3 emoji", "😀😀😀", invalid},
		{"emoji after letters", "ana😀", "Ana😀"},
		{"no upper case form", "ßtest", "ßtest"},
		{"leading and trailing spaces", "   bruno  ", "Bruno"},
		{"spaces do not count", "  ana  ", invalid},
		{"multiple inner spaces", "ana    maria", "Ana maria"},
		{"tabs, newlines and nbsp", "\tana\n  maria ", "Ana maria"},
		{"collapsed before counting", "a   b", invalid},
		{"only spaces", "     ", invalid},
		{
			"24 runes after collapsing",
			"abcdefghijk" + strings.Repeat(" ", 10) + "lmnopqrstuvw",
			"Abcdefghijk lmnopqrstuvw",
		},
		{"already capitalized", "Maria", "Maria"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := requests.NameSchema().Parse(tc.in)

			if tc.want == invalid {
				issues, ok := z.AsIssues(err)
				if !ok || len(issues) != 1 || issues[0].Message != requests.MsgNameLength {
					t.Fatalf("Parse(%q) = %q, %v; want the %s issue", tc.in, got, err, requests.MsgNameLength)
				}

				return
			}

			if err != nil || got != tc.want {
				t.Fatalf("Parse(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
			}
		})
	}
}
