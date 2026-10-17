package catalog

import (
	"image"
	"image/color"
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

func TestDownscale(t *testing.T) {
	t.Parallel()

	src := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for y := range 200 {
		for x := range 400 {
			c := uint8(0)
			if x%2 == 0 {
				c = 255
			}

			src.SetRGBA(x, y, color.RGBA{R: c, G: c, B: c, A: 255})
		}
	}

	dst := downscale(src, 100)
	if dst.Rect.Dx() != 100 || dst.Rect.Dy() != 50 {
		t.Fatalf("size = %v, want 100x50", dst.Rect)
	}

	// Alternating black/white columns average to mid grey.
	if got := dst.RGBAAt(10, 10).R; got < 120 || got > 135 {
		t.Fatalf("pixel = %d, want ~127", got)
	}

	if downscale(src, 400) != src || downscale(src, 1000) != src {
		t.Fatal("images at most w wide must be returned as is")
	}
}
