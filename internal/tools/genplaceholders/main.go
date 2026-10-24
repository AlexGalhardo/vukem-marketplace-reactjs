//go:build ignore

// Command genplaceholders draws the demo catalog's category images: a
// gradient with one simple white glyph, encoded as small WebP files with
// ffmpeg (libwebp). The output is committed under app/catalog/assets/shared
// and embedded in the binary, so demo images never depend on disk or network.
//
//	go run internal/tools/genplaceholders/main.go
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	canvas = 1280 // drawn large, scaled down by ffmpeg for anti-aliasing
	outPx  = 640
	outDir = "app/catalog/assets/shared"
)

type glyph struct {
	slug     string
	from, to color.RGBA
	draw     func(c *canvasT)
}

type canvasT struct{ img *image.RGBA }

var white = color.RGBA{R: 255, G: 255, B: 255, A: 255}

func (c *canvasT) rect(x0, y0, x1, y1 int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c.img.SetRGBA(x, y, white)
		}
	}
}

func (c *canvasT) disc(cx, cy, r, inner int) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			d := (x-cx)*(x-cx) + (y-cy)*(y-cy)
			if d <= r*r && d >= inner*inner {
				c.img.SetRGBA(x, y, white)
			}
		}
	}
}

func (c *canvasT) poly(pts ...[2]int) {
	for y := 0; y < canvas; y++ {
		for x := 0; x < canvas; x++ {
			in := false
			for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
				if (pts[i][1] > y) != (pts[j][1] > y) &&
					float64(
						x,
					) < float64(
						pts[j][0]-pts[i][0],
					)*float64(
						y-pts[i][1],
					)/float64(
						pts[j][1]-pts[i][1],
					)+float64(
						pts[i][0],
					) {
					in = !in
				}
			}
			if in {
				c.img.SetRGBA(x, y, white)
			}
		}
	}
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 255} }

var glyphs = []glyph{
	{"electronics", rgb(17, 24, 39), rgb(107, 114, 128), func(c *canvasT) {
		c.disc(640, 640, 300, 250)
		c.rect(300, 600, 340, 640)
		c.rect(300, 640, 420, 900)
		c.rect(860, 640, 980, 900)
	}},
	{"phones", rgb(30, 64, 175), rgb(96, 165, 250), func(c *canvasT) {
		c.rect(460, 240, 820, 1040)
		c.disc(640, 960, 28, 0)
	}},
	{"computers", rgb(55, 65, 81), rgb(156, 163, 175), func(c *canvasT) {
		c.rect(300, 340, 980, 820)
		c.rect(200, 860, 1080, 920)
	}},
	{"home", rgb(68, 64, 60), rgb(253, 230, 138), func(c *canvasT) {
		c.poly([2]int{440, 480}, [2]int{840, 480}, [2]int{960, 300}, [2]int{320, 300})
		c.rect(620, 480, 660, 960)
		c.rect(480, 960, 800, 1010)
	}},
	{"appliances", rgb(127, 29, 29), rgb(248, 113, 113), func(c *canvasT) {
		c.rect(380, 540, 800, 920)
		c.disc(850, 700, 120, 80)
		c.rect(440, 360, 480, 480)
		c.rect(620, 360, 660, 480)
	}},
	{"fashion", rgb(146, 64, 14), rgb(251, 191, 36), func(c *canvasT) {
		c.poly([2]int{440, 280}, [2]int{840, 280}, [2]int{1020, 480}, [2]int{900, 580},
			[2]int{840, 520}, [2]int{840, 1000}, [2]int{440, 1000}, [2]int{440, 520},
			[2]int{380, 580}, [2]int{260, 480})
	}},
	{"sports", rgb(6, 95, 70), rgb(52, 211, 153), func(c *canvasT) {
		c.rect(300, 520, 380, 760)
		c.rect(220, 580, 300, 700)
		c.rect(380, 620, 900, 660)
		c.rect(900, 520, 980, 760)
		c.rect(980, 580, 1060, 700)
	}},
	{"beauty", rgb(157, 23, 77), rgb(244, 114, 182), func(c *canvasT) {
		c.rect(500, 480, 780, 1000)
		c.rect(580, 360, 700, 480)
		c.rect(560, 280, 720, 360)
	}},
	{"toys", rgb(161, 98, 7), rgb(250, 204, 21), func(c *canvasT) {
		c.rect(320, 700, 640, 960)
		c.rect(660, 700, 980, 960)
		c.rect(490, 420, 810, 680)
	}},
	{"tools", rgb(30, 58, 138), rgb(251, 146, 60), func(c *canvasT) {
		c.rect(600, 440, 680, 1040)
		c.rect(380, 280, 900, 440)
	}},
	{"books", rgb(8, 47, 73), rgb(56, 189, 248), func(c *canvasT) {
		c.rect(260, 340, 620, 940)
		c.rect(660, 340, 1020, 940)
	}},
	{"car-parts", rgb(63, 63, 70), rgb(161, 161, 170), func(c *canvasT) {
		c.disc(640, 640, 360, 250)
		c.disc(640, 640, 90, 0)
		for _, a := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2} {
			x, y := 640+int(170*math.Cos(a)), 640+int(170*math.Sin(a))
			c.disc(x, y, 40, 0)
		}
	}},
}

func main() {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(err)
	}

	tmp, err := os.MkdirTemp("", "genplaceholders")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(tmp)

	for _, g := range glyphs {
		img := image.NewRGBA(image.Rect(0, 0, canvas, canvas))
		for y := range canvas {
			for x := range canvas {
				t := float64(x+y) / float64(2*canvas)
				img.SetRGBA(x, y, color.RGBA{
					R: lerp(g.from.R, g.to.R, t), G: lerp(g.from.G, g.to.G, t), B: lerp(g.from.B, g.to.B, t), A: 255,
				})
			}
		}

		g.draw(&canvasT{img: img})

		src := filepath.Join(tmp, g.slug+".png")
		f, err := os.Create(src)
		if err != nil {
			fail(err)
		}
		if err := png.Encode(f, img); err != nil {
			fail(err)
		}
		f.Close()

		dst := filepath.Join(outDir, g.slug+".webp")
		out, err := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-i", src,
			"-vf", fmt.Sprintf("scale=%d:%d:flags=lanczos", outPx, outPx),
			"-c:v", "libwebp", "-quality", "55", "-compression_level", "6", dst).CombinedOutput()
		if err != nil {
			fail(fmt.Errorf("ffmpeg %s: %w\n%s", g.slug, err, out))
		}
	}
}

func lerp(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
