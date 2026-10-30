package designcompare

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // references may be photos
	"image/png"
	"math"
	"sort"

	_ "golang.org/x/image/webp" // and pasted WebP

	"github.com/enowx/enowx/proto"
)

// Squint is how alike two first screens' light and dark shapes are, 0 to
// 100: each cut to its first screen (16:10 of its width), shrunk to a few
// blocks of gray, compared.
func Squint(a, b []byte) (int, error) {
	ga, err := squinted(a)
	if err != nil {
		return 0, err
	}
	gb, err := squinted(b)
	if err != nil {
		return 0, err
	}
	// Correlation of the two grids, light and dark shapes alike from 0.
	var ma, mb float64
	for i := range ga {
		ma += ga[i]
		mb += gb[i]
	}
	ma /= float64(len(ga))
	mb /= float64(len(gb))
	var num, da, db float64
	for i := range ga {
		num += (ga[i] - ma) * (gb[i] - mb)
		da += (ga[i] - ma) * (ga[i] - ma)
		db += (gb[i] - mb) * (gb[i] - mb)
	}
	if da == 0 || db == 0 {
		if da == db {
			return 100, nil // both flat
		}
		return 0, nil
	}
	r := num / math.Sqrt(da*db)
	return int(math.Round(max(r, 0) * 100)), nil
}

const squintW, squintH = 16, 10

// squinted is a picture's first screen as squintW×squintH grays.
func squinted(data []byte) ([]float64, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	h := min(b.Dy(), b.Dx()*10/16)
	if b.Dx() == 0 || h == 0 {
		return nil, errors.New("designcompare: an empty picture")
	}
	out := make([]float64, squintW*squintH)
	n := make([]int, len(out))
	step := max(1, min(b.Dx(), h)/200) // a few hundred samples a side is plenty
	for y := 0; y < h; y += step {
		for x := 0; x < b.Dx(); x += step {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := (y*squintH/h)*squintW + x*squintW/b.Dx()
			out[i] += (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(bl)) / 65535
			n[i]++
		}
	}
	for i := range out {
		if n[i] > 0 {
			out[i] /= float64(n[i])
		}
	}
	return out, nil
}

// matchW is the width pictures are compared at.
const matchW = 480

var cellNames = [9]string{"top left", "top", "top right", "left", "middle", "right", "bottom left", "bottom", "bottom right"}

// Match compares design (a render of it) with the picture it recreates,
// at one width, over their common height.
func Match(design, reference []byte) (proto.CanvasMatch, error) {
	da, _, err := image.Decode(bytes.NewReader(design))
	if err != nil {
		return proto.CanvasMatch{}, err
	}
	ra, _, err := image.Decode(bytes.NewReader(reference))
	if err != nil {
		return proto.CanvasMatch{}, err
	}
	d, r := scaled(da), scaled(ra)
	h := min(d.Bounds().Dy(), r.Bounds().Dy())
	if h == 0 {
		return proto.CanvasMatch{}, errors.New("designcompare: an empty picture")
	}
	var m proto.CanvasMatch
	var cell, all [9]int
	overlay := image.NewRGBA(image.Rect(0, 0, matchW, h))
	differ := 0
	for y := range h {
		for x := range matchW {
			c1, c2 := d.RGBAAt(x, y), r.RGBAAt(x, y)
			i := (y*3/h)*3 + x*3/matchW
			all[i]++
			if far(c1, c2) {
				differ++
				cell[i]++
			}
			overlay.SetRGBA(x, y, color.RGBA{uint8((int(c1.R) + int(c2.R)) / 2), uint8((int(c1.G) + int(c2.G)) / 2), uint8((int(c1.B) + int(c2.B)) / 2), 255})
		}
	}
	m.Diff = round1(100 * float64(differ) / float64(matchW*h))
	order := make([]int, 9)
	for i := range 9 {
		m.Cells[i] = round1(100 * float64(cell[i]) / float64(max(all[i], 1)))
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return m.Cells[order[a]] > m.Cells[order[b]] })
	for _, i := range order[:3] {
		if m.Cells[i] > 0 {
			m.Worst = append(m.Worst, cellNames[i])
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, overlay); err == nil {
		m.Overlay = buf.Bytes()
	}
	return m, nil
}

// far says two pixels differ: a channel by more than a tenth.
func far(a, b color.RGBA) bool {
	d := func(x, y uint8) bool { return math.Abs(float64(x)-float64(y)) > 25.5 }
	return d(a.R, b.R) || d(a.G, b.G) || d(a.B, b.B)
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

// scaled is img at matchW wide, its shape kept (nearest pixel).
func scaled(img image.Image) *image.RGBA {
	b := img.Bounds()
	h := max(1, b.Dy()*matchW/max(b.Dx(), 1))
	src := image.NewRGBA(b)
	draw.Draw(src, b, img, b.Min, draw.Src)
	out := image.NewRGBA(image.Rect(0, 0, matchW, h))
	for y := range h {
		for x := range matchW {
			out.SetRGBA(x, y, src.RGBAAt(b.Min.X+x*b.Dx()/matchW, b.Min.Y+y*b.Dy()/h))
		}
	}
	return out
}
