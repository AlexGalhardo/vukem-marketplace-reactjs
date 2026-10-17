package catalog

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"Câmera Fotográfica", "camera-fotografica"},
		{"  Ação & Reação!! ", "acao-reacao"},
		{"iPhone 13 — 128GB", "iphone-13-128gb"},
		{"Coração ÇÃO ñ ß", "coracao-cao-n-ss"},
		{"../../etc/passwd", "etc-passwd"},
		{"<script>alert(1)</script>", "script-alert-1-script"},
		{"日本語", "item"},
		{"", "item"},
		{strings.Repeat("a", 200), strings.Repeat("a", maxSlugLen)},
		{strings.Repeat("ab ", 50), strings.TrimRight(strings.Repeat("ab-", 24), "-")},
	}

	for _, tt := range tests {
		if got := Slugify(tt.in); got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMediaObject(t *testing.T) {
	t.Parallel()

	name := strings.Repeat("a2", 13) // upload.Receive names: 26 base32 chars

	tests := []struct {
		path, ctype string
	}{
		{"products/12/" + name + ".jpg", "image/jpeg"},
		{"products/12/" + name + "-w640.png", "image/png"},
		{"shops/1/" + name + ".webp", "image/webp"},
		{"shops/1/" + name + "-w320.gif", "image/gif"},
		{"products/12/" + name + "-w641.jpg", ""},
		{"products/12/" + name + ".svg", ""},
		{"products/12/" + name + ".html", ""},
		{"users/12/" + name + ".jpg", ""},
		{"products/x/" + name + ".jpg", ""},
		{"products/12/" + strings.ToUpper(name) + ".jpg", ""},
		{"products/12/../" + name + ".jpg", ""},
		{"products/12/" + name + ".jpg/../../.env", ""},
		{"quarantine/." + name + ".jpg", ""},
	}

	for _, tt := range tests {
		ctype, ok := MediaObject(tt.path)
		if ctype != tt.ctype || ok != (tt.ctype != "") {
			t.Errorf("MediaObject(%q) = %q, %v; want %q", tt.path, ctype, ok, tt.ctype)
		}
	}

	if got := variantPath("products/12/"+name+".png", 320); got != "products/12/"+name+"-w320.png" {
		t.Errorf("variantPath = %q", got)
	}
}
