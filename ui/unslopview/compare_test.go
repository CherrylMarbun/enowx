package unslopview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"
)

func solid(t *testing.T, c color.RGBA) *ui.Bitmap {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	bmp, err := ui.DecodeBitmap(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return bmp
}

// The before shows left of the divider, the after right of it; dragging
// moves the divider, and Close closes.
func TestCompareDrags(t *testing.T) {
	red, blue := solid(t, color.RGBA{255, 0, 0, 255}), solid(t, color.RGBA{0, 0, 255, 255})
	var v Compare
	closed := false
	o := CompareOptions{Shots: []Shot{{Label: "As shown", Before: red, After: blue}}, From: 40, To: 81, Close: func() { closed = true }}
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, o) }, 1000, 600)
	tt.Frame()
	r, ok := tt.Find("Drag to compare before and after")
	if !ok || !tt.HasText("Before") || !tt.HasText("After") || !tt.HasText("81") {
		t.Fatalf("no comparison: %v %q", ok, tt.Texts())
	}
	at := func(f float32) color.RGBA {
		return tt.Image().RGBAAt(int(r.X+r.W*f), int(r.Y+r.H*0.8))
	}
	if left, right := at(0.3), at(0.7); left.R < 200 || right.B < 200 {
		t.Fatalf("at the middle: left %v, right %v", left, right)
	}
	tt.Press(r.X+r.W*0.5, r.Y+r.H/2)
	tt.Frame()
	tt.Move(r.X+r.W*0.9, r.Y+r.H/2)
	tt.Frame()
	tt.Release(r.X+r.W*0.9, r.Y+r.H/2)
	tt.Frame()
	shot(t, tt, "compare-slider.png")
	if mid := at(0.7); mid.R < 200 {
		t.Fatalf("dragged right, the before should show at 70%%: %v", mid)
	}
	if err := tt.Click("Close"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !closed {
		t.Fatal("not closed")
	}
}

// With a shot per size, the sizes are tabs: drawn untouched, and a click
// on one shows it.
func TestCompareSizes(t *testing.T) {
	red, blue := solid(t, color.RGBA{255, 0, 0, 255}), solid(t, color.RGBA{0, 0, 255, 255})
	var v Compare
	o := CompareOptions{Shots: []Shot{{Label: "As shown", Before: red, After: blue}, {Label: "Tablet", Before: red, After: blue}, {Label: "Phone", Before: blue, After: red}}, From: 40, To: 71}
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, o) }, 1000, 600)
	tt.Frame()
	tt.Frame()
	if v.size != 0 {
		t.Fatalf("size %d untouched", v.size)
	}
	if err := tt.Click("Phone"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if v.size != 2 {
		t.Fatalf("size %d after Phone", v.size)
	}
}
