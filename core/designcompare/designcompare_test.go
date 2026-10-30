package designcompare

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/enowx/enowx/proto"
)

// pic is a w×h picture, light, with dark blocks where rects say.
func pic(t *testing.T, w, h int, rects ...image.Rectangle) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, color.RGBA{240, 240, 240, 255})
			for _, r := range rects {
				if (image.Point{x, y}).In(r) {
					img.SetRGBA(x, y, color.RGBA{20, 20, 20, 255})
				}
			}
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// Two directions may share one of skeleton, type, palette and hero; a
// first screen with its dark block left reads alike another's with it
// left, not with it right.
func TestDirectionsDiffer(t *testing.T) {
	a := proto.DesignDirection{Skeleton: "Split: copy left, product shot right", Type: "Inter, 400/600", Palette: []string{"#0b0b0f", "#7c3aed"}, Hero: "A product screenshot"}
	b := proto.DesignDirection{Skeleton: "Split: copy left, chart right", Type: "Inter 500", Palette: []string{"#111111", "#8b5cf6"}, Hero: "A chart of usage"}
	if s := Shared(a, b); len(s) != 3 || s[0] != "skeleton" || s[1] != "type" || s[2] != "palette" {
		t.Fatalf("shared: %v", s)
	}
	c := proto.DesignDirection{Skeleton: "Full-bleed type over a ledger", Type: "Fraunces, 300", Palette: []string{"#f4efe6", "#c2410c"}, Hero: "The ledger itself"}
	if s := Shared(a, c); len(s) != 0 {
		t.Fatalf("a and c: %v", s)
	}
	left := pic(t, 320, 200, image.Rect(0, 0, 150, 200))
	left2 := pic(t, 640, 400, image.Rect(0, 0, 290, 400))
	right := pic(t, 320, 200, image.Rect(170, 0, 320, 200))
	same, _ := Squint(left, left2)
	other, _ := Squint(left, right)
	if same < TooAlike || other > 20 {
		t.Fatalf("squint: alike %d, unlike %d", same, other)
	}
	if p := Pair("A", "B", a, b, same); !TooClose(p) || p.Distinct > 30 {
		t.Fatalf("a and b: %+v", p)
	}
	if p := Pair("A", "C", a, c, other); TooClose(p) || p.Distinct < 90 {
		t.Fatalf("a and c: %+v", p)
	}
}

// A design matching its reference but for one corner says so, that
// corner worst.
func TestMatch(t *testing.T) {
	ref := pic(t, 960, 600, image.Rect(0, 0, 960, 80))
	design := pic(t, 480, 300, image.Rect(0, 0, 480, 40), image.Rect(380, 220, 480, 300))
	m, err := Match(design, ref)
	if err != nil {
		t.Fatal(err)
	}
	if m.Diff <= 0 || m.Diff > 10 || len(m.Worst) == 0 || m.Worst[0] != "bottom right" || m.Cells[0] != 0 || len(m.Overlay) == 0 {
		t.Fatalf("%+v", m.Worst)
	}
}

// A card as an agent writes it reads back, and one lacking says what.
func TestCard(t *testing.T) {
	d, err := ParseCard("North star: a ledger you trust\n- Source: an accountant's ledger, ruled paper\nSkeleton: full-bleed type\nType: Fraunces 300\nPalette: #f4efe6 paper, #c2410c ink\nMoment: the total settles")
	if err != nil || d.Source != "an accountant's ledger, ruled paper" || len(d.Palette) != 2 || d.Palette[1] != "#c2410c" {
		t.Fatalf("%+v %v", d, err)
	}
	if back, err := ParseCard(CardText(d)); err != nil || back.Moment != d.Moment || len(back.Palette) != 2 {
		t.Fatalf("round trip: %+v %v", back, err)
	}
	if _, err := ParseCard("North star: x"); err == nil {
		t.Fatal("a card lacking was taken")
	}
}
