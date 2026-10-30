package ui

import (
	"bytes"
	"image"
	"image/png"
	"net"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// A Browser's page goes on a canvas as a design named Current, at the
// page's size (a 2x screenshot halved), its picture filling it; a new
// canvas named after the page when none is open.
func TestPageToCanvas(t *testing.T) {
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	a, b := net.Pipe()
	go srv.ServeConn(a)
	cl, err := client.New(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	var shot bytes.Buffer
	png.Encode(&shot, image.NewRGBA(image.Rect(0, 0, 2560, 1600)))
	id, frame, err := putPage(cl, "", shot.Bytes(), "http://localhost:5173/pricing/", 1280)
	if err != nil {
		t.Fatal(err)
	}
	cv, err := cl.Canvas(id)
	if err != nil {
		t.Fatal(err)
	}
	f := cv.Frames[len(cv.Frames)-1]
	if cv.Title != "localhost:5173/pricing" || f.ID != frame || f.Name != "Current" || f.Width != 1280 || f.Height != 800 {
		t.Fatalf("canvas %q, design %+v", cv.Title, f)
	}
	if len(f.Elements) != 1 || f.Elements[0].Type != proto.ShapeImage || f.Elements[0].W != 1280 || f.Elements[0].Src == "" {
		t.Fatalf("its picture: %+v", f.Elements)
	}
	if data, err := cl.CanvasAsset(id, f.Elements[0].Src); err != nil || !bytes.Equal(data, shot.Bytes()) {
		t.Fatalf("the asset: %v", err)
	}
}
