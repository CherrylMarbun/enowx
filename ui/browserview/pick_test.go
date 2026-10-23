package browserview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// A capture at twice the page's pixels is cut to the element, with room
// around it.
func TestPickedCropsToTheElement(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	for y := 80; y < 162; y++ {
		for x := 80; x < 256; x++ {
			img.Set(x, y, color.RGBA{37, 99, 235, 255})
		}
	}
	var capture bytes.Buffer
	if err := png.Encode(&capture, img); err != nil {
		t.Fatal(err)
	}
	p := Picked{URL: "http://localhost:5173/", Selector: "#buy", HTML: `<button id="buy">Buy now</button>`,
		Styles: "color: rgb(255, 255, 255)", Rect: [4]float64{40, 40, 88, 41}, Viewport: 600}
	shot, err := p.Crop(capture.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cut, err := png.Decode(bytes.NewReader(shot))
	if err != nil {
		t.Fatal(err)
	}
	if b := cut.Bounds(); b.Dx() != (88+16)*2 || b.Dy() != (41+16)*2 {
		t.Fatalf("cut %v", b)
	}
	about := p.Describe()
	for _, want := range []string{"http://localhost:5173/", "#buy", "Buy now", "color: rgb(255, 255, 255)"} {
		if !strings.Contains(about, want) {
			t.Fatalf("description lacks %q:\n%s", want, about)
		}
	}
}
