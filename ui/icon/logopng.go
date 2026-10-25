package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"strconv"
)

// The logo is pixel art on a 128 grid, rows of runs ("Mx yhWv1h-Wz"):
// LogoPNG draws it at any size, for icons outside the app's own views (the
// tray's).

var logoRun = regexp.MustCompile(`M(\d+) (\d+)h(\d+)v1h-\d+z`)

// LogoPNG is the logo as a size×size PNG in ink, on transparent.
func LogoPNG(size int, ink color.NRGBA) []byte {
	data, _ := files.ReadFile("svg/logo.svg")
	// Each pixel's coverage of the 128 grid's cells, averaged.
	var grid [128][128]bool
	for _, m := range logoRun.FindAllSubmatch(data, -1) {
		x, _ := strconv.Atoi(string(m[1]))
		y, _ := strconv.Atoi(string(m[2]))
		w, _ := strconv.Atoi(string(m[3]))
		for i := x; i < x+w && i < 128; i++ {
			if y < 128 {
				grid[y][i] = true
			}
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := range size {
		for px := range size {
			y0, y1 := py*128/size, (py+1)*128/size
			x0, x1 := px*128/size, (px+1)*128/size
			on, all := 0, 0
			for y := y0; y < max(y1, y0+1); y++ {
				for x := x0; x < max(x1, x0+1); x++ {
					all++
					if grid[y][x] {
						on++
					}
				}
			}
			if on > 0 {
				c := ink
				c.A = uint8(int(ink.A) * on / all)
				img.SetNRGBA(px, py, c)
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
