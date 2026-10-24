// Command appicon draws the app's icons from the mark the sidebar shows
// (ui/icon/svg/logo.svg), as the sidebar shows it: the mark on a rounded
// square, white, laid out on Apple's icon grid, 1024 pixels wide. The development
// app's (icon-dev.png) is blue, to tell the two apart in the Dock.
//
//	go run ./scripts/appicon
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

const (
	size  = 1024
	body  = 824 // the rounded square, as Apple's icon grid
	inset = (size - body) / 2
	ss    = 4 // samples per pixel side, for smooth edges
)

type rect struct{ x, y, w, h float64 }

func main() {
	marks, err := readMark("ui/icon/svg/logo.svg")
	if err != nil {
		log.Fatal(err)
	}
	for name, colors := range map[string][3]color.RGBA{
		// White, the mark in the dark of the app's text.
		"icon.png": {{0xFF, 0xFF, 0xFF, 255}, {0xEC, 0xEC, 0xEE, 255}, {0x16, 0x15, 0x13, 255}},
		// The development app, blue.
		"icon-dev.png": {{0xA4, 0xC0, 0xFF, 255}, {0x7F, 0xA6, 0xFF, 255}, {0x0E, 0x0F, 0x12, 255}},
	} {
		img := draw(marks, colors[0], colors[1], colors[2])
		out := filepath.Join("cmd", "enowx", "resources", name)
		f, err := os.Create(out)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			log.Fatal(err)
		}
		f.Close()
		fmt.Println(out)
	}
}

// readMark reads the mark's rectangles (it is drawn of them) in its
// 128-unit box.
func readMark(path string) ([]rect, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`M(\d+) (\d+)h(\d+)v(\d+)h-\d+z`)
	var out []rect
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		f := func(i int) float64 { v, _ := strconv.Atoi(m[i]); return float64(v) }
		out = append(out, rect{f(1), f(2), f(3), f(4)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no rectangles in %s", path)
	}
	return out, nil
}

func draw(marks []rect, top, bottom, ink color.RGBA) *image.RGBA {
	// The mark's 128-unit box, 28 points of the 36 the sidebar gives it,
	// centered.
	scale := body * 28.0 / 36 / 128
	ox, oy := size/2-64*scale, size/2-64*scale
	inMark := func(x, y float64) bool {
		ux, uy := (x-ox)/scale, (y-oy)/scale
		for _, r := range marks {
			if ux >= r.x && ux < r.x+r.w && uy >= r.y && uy < r.y+r.h {
				return true
			}
		}
		return false
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := float64(px) + (float64(sx)+0.5)/ss
					y := float64(py) + (float64(sy)+0.5)/ss
					// A soft shadow under the square.
					if !inSquircle(x, y) {
						if d := squircleDistance(x, y-12); d < 28 {
							a += 0.28 * (1 - d/28) * (1 - d/28)
						}
						continue
					}
					t := (y - inset) / body
					c := mix(top, bottom, t)
					if inMark(x, y) {
						c = ink
					}
					r, g, b, a = r+float64(c.R), g+float64(c.G), b+float64(c.B), a+1
				}
			}
			n := float64(ss * ss)
			if a == 0 {
				continue
			}
			covered := math.Min(a, n)
			// Premultiplied: the shadow is black.
			img.SetRGBA(px, py, color.RGBA{
				R: uint8(r / n), G: uint8(g / n), B: uint8(b / n),
				A: uint8(math.Round(255 * covered / n)),
			})
		}
	}
	return img
}

// inSquircle reports whether (x, y) is inside the square, its corners
// rounded as the sidebar's mark's (8 points of 36).
func inSquircle(x, y float64) bool { return squircleDistance(x, y) == 0 }

// squircleDistance is how far (x, y) is outside the square, in pixels.
func squircleDistance(x, y float64) float64 {
	const half, radius = body / 2.0, body * 8.0 / 36
	dx := math.Max(math.Abs(x-size/2)-(half-radius), 0)
	dy := math.Max(math.Abs(y-size/2)-(half-radius), 0)
	return math.Max(math.Hypot(dx, dy)-radius, 0)
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	t = math.Min(math.Max(t, 0), 1)
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}
