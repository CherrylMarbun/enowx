package daemon

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A canvas shared by a link is served read-only to whoever has it: its
// page, a version that moves when it changes; the link stops on asking,
// and with the canvas.
func TestCanvasSharedByALink(t *testing.T) {
	srv := startServer(t)
	cl := connect(t, srv, nil)
	cv, err := cl.CreateCanvas(proto.CanvasCreateParams{Title: "Landing", Kind: "mockup"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cl.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, HTML: `<h1 id="title">Fresh coffee</h1>`}); err != nil {
		t.Fatal(err)
	}
	if list, _ := cl.CanvasShares(); len(list) != 0 {
		t.Fatalf("shared before asked: %+v", list)
	}
	x, err := cl.ShareCanvas(cv.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.URLs) == 0 || !strings.HasPrefix(x.URLs[0], "http://127.0.0.1:") {
		t.Fatalf("link: %+v", x)
	}
	again, _ := cl.ShareCanvas(cv.ID, false)
	if again.Token != x.Token {
		t.Fatal("sharing again made another link")
	}
	get := func(url string) (int, string) {
		t.Helper()
		res, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}
	code, page := get(x.URLs[0])
	if code != 200 || !strings.Contains(page, "Fresh coffee") {
		t.Fatalf("page: %d %.200s", code, page)
	}
	_, v1 := get(x.URLs[0] + "/version")
	if _, err := cl.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLPatch, Target: "title", HTML: `<h1 id="title">Fresh tea</h1>`}); err != nil {
		t.Fatal(err)
	}
	if _, v2 := get(x.URLs[0] + "/version"); v2 == v1 {
		t.Fatal("the version didn't move with the canvas")
	}
	if code, _ := get(strings.Replace(x.URLs[0], x.Token, strings.Repeat("0", 32), 1)); code != 404 {
		t.Fatalf("another token: %d", code)
	}
	if _, err := cl.ShareCanvas(cv.ID, true); err != nil {
		t.Fatal(err)
	}
	if list, _ := cl.CanvasShares(); len(list) != 0 {
		t.Fatalf("still shared: %+v", list)
	}
	if _, err := http.Get(x.URLs[0]); err == nil {
		t.Fatal("still served after its only link stopped")
	}
	// A deleted canvas takes its link with it.
	if _, err := cl.ShareCanvas(cv.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := cl.DeleteCanvas(cv.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := cl.CanvasShares(); len(list) != 0 {
		t.Fatalf("a deleted canvas's link stays: %+v", list)
	}
}
