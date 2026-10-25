package canvas

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

func ptr[T any](v T) *T { return &v }

// An agent writes a mockup an element at a time; the user's change to
// another element meanwhile stays, undo and redo step through them, and
// restoring an old version makes a new one.
func TestCanvasEditedAnElementAtATime(t *testing.T) {
	s := Open(t.TempDir())
	c, err := s.Create(proto.CanvasCreateParams{Title: "Dashboard"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != proto.CanvasMockup || len(c.Frames) != 1 || c.Frames[0].Width != 1440 || c.Version != 1 {
		t.Fatalf("made %+v", c)
	}
	edit := func(e proto.CanvasEdit) proto.Canvas {
		t.Helper()
		e.ID = c.ID
		got, _, _, err := s.Edit(e)
		if err != nil {
			t.Fatalf("%s: %v", e.Op, err)
		}
		return got
	}
	edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLSet, HTML: `<style>body{font:14px sans-serif}</style><main id="app"></main>`, Session: "s1"})
	edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLPatch, Into: "app", HTML: `<nav id="nav">Home</nav>`, Session: "s1"})
	edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLPatch, Into: "app", HTML: `<section id="stats"><h2 id="title">Stats</h2></section>`, Session: "s1"})
	// The user retitles meanwhile; the agent then changes the nav.
	edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLText, Target: "title", HTML: "Revenue"})
	got := edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLPatch, Target: "nav", HTML: `<nav id="nav">Home · Reports</nav>`, Session: "s1"})
	page := got.Frames[0].HTML
	for _, want := range []string{"<style>", `<nav id="nav">Home · Reports</nav>`, `<h2 id="title">Revenue</h2>`} {
		if !strings.Contains(page, want) {
			t.Fatalf("page lacks %q:\n%s", want, page)
		}
	}
	ids := []string{}
	for _, h := range Outline(page) {
		ids = append(ids, h.ID)
	}
	if strings.Join(ids, ",") != "app,nav,stats,title" {
		t.Fatalf("outline %v", ids)
	}
	if got.Agent != "s1" || got.Version != 6 {
		t.Fatalf("agent %q version %d", got.Agent, got.Version)
	}

	undone, err := s.Undo(c.ID)
	if err != nil || !strings.Contains(undone.Frames[0].HTML, `<nav id="nav">Home</nav>`) || undone.Version != 5 {
		t.Fatalf("undo: %v %+v", err, undone.Frames[0].HTML)
	}
	redone, err := s.Redo(c.ID)
	if err != nil || redone.Version != 6 {
		t.Fatalf("redo: %v %d", err, redone.Version)
	}
	if _, err := s.Redo(c.ID); err == nil {
		t.Fatal("redo past the newest")
	}
	changes := Compare(mustVersion(t, s, c.ID, 2), redone)
	var said []string
	for _, ch := range changes {
		said = append(said, ch.String())
	}
	if len(said) != 3 || !strings.Contains(strings.Join(said, "\n"), "added: #nav") {
		t.Fatalf("compare: %q", said)
	}
	restored, err := s.Restore(c.ID, 2)
	if err != nil || restored.Version != 7 || strings.Contains(restored.Frames[0].HTML, "nav") {
		t.Fatalf("restore: %v %+v", err, restored)
	}
	h, _ := s.History(c.ID)
	if h.Head != 7 || len(h.Versions) != 7 || h.Versions[4].Element != "title" || h.Versions[1].Session != "s1" {
		t.Fatalf("history %+v", h)
	}
}

func mustVersion(t *testing.T, s *Store, id string, n int) proto.Canvas {
	t.Helper()
	c, err := s.Version(id, n)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// A diagram's shapes are drawn by the daemon itself, arrows between boxes
// included; a frame with HTML needs the app's web engine.
func TestDiagramDrawnWithoutABrowser(t *testing.T) {
	s := Open(t.TempDir())
	c, _ := s.Create(proto.CanvasCreateParams{Title: "Flow", Kind: proto.CanvasDiagram})
	for _, el := range []proto.CanvasElement{
		{ID: "a", Type: proto.ShapeRect, X: 40, Y: 40, W: 200, H: 80, Text: "Client", Fill: "#dbeafe", Radius: 10},
		{ID: "b", Type: proto.ShapeDiamond, X: 400, Y: 30, W: 160, H: 100, Text: "Auth?"},
		{Type: proto.ShapeArrow, From: "a", To: "b", Text: "login"},
	} {
		if _, _, _, err := s.Edit(proto.CanvasEdit{ID: c.ID, Op: proto.CanvasOpElementSet, Element: &el}); err != nil {
			t.Fatal(err)
		}
	}
	c, _ = s.Get(c.ID)
	if c.Frames[0].Elements[2].ID != "arrow-3" {
		t.Fatalf("arrow named %q", c.Frames[0].Elements[2].ID)
	}
	data, err := PNG(c.Frames[0], nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != 1600 {
		t.Fatalf("png: %v", err)
	}
	if r, g, b, _ := img.At(60, 80).RGBA(); r>>8 != 0xdb || g>>8 != 0xea || b>>8 != 0xfe {
		t.Fatalf("box drawn %x %x %x", r>>8, g>>8, b>>8)
	}
	svg := SVG(c.Frames[0], nil, false)
	if !strings.Contains(svg, `marker-end="url(#arrow)"`) || !strings.Contains(svg, ">Client<") {
		t.Fatalf("svg %s", svg)
	}
	pdf, err := PDF(Raster(c.Frames[0], nil, 1))
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatalf("pdf %v", err)
	}
	if _, _, _, err := s.Edit(proto.CanvasEdit{ID: c.ID, Op: proto.CanvasOpElementDel, Target: "a"}); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Get(c.ID)
	if len(c.Frames[0].Elements) != 1 {
		t.Fatalf("the arrow to a stayed: %+v", c.Frames[0].Elements)
	}
	c.Frames[0].HTML = "<p>hi</p>"
	if _, err := PNG(c.Frames[0], nil, 1); err != ErrNeedsBrowser {
		t.Fatalf("html drawn: %v", err)
	}
	doc := Standalone(c, nil)
	if !strings.Contains(doc, "read-only") || !strings.Contains(doc, "srcdoc=") {
		t.Fatal("standalone page")
	}
}

func TestCommentsAndAssets(t *testing.T) {
	s := Open(t.TempDir())
	c, _ := s.Create(proto.CanvasCreateParams{Title: "Board", Kind: proto.CanvasBoard})
	list, err := s.Comment(proto.CanvasCommentParams{ID: c.ID, Op: proto.CanvasCommentAdd, Comment: proto.CanvasComment{Element: "nav", Text: "Make it sticky"}})
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	list, _ = s.Comment(proto.CanvasCommentParams{ID: c.ID, Op: proto.CanvasCommentResolve, Comment: proto.CanvasComment{ID: list[0].ID}})
	if !list[0].Resolved {
		t.Fatal("not resolved")
	}
	if _, err := s.AddAsset(c.ID, "../../etc/logo.png", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if data, err := s.Asset(c.ID, "logo.png"); err != nil || string(data) != "x" {
		t.Fatalf("asset %q %v", data, err)
	}
	if _, err := s.Asset(c.ID, "../canvas.json"); err == nil {
		t.Fatal("read outside its assets")
	}
	if _, _, _, err := s.Edit(proto.CanvasEdit{ID: c.ID, Op: proto.CanvasOpMeta, Title: ptr("Ideas"), Projects: ptr([]string{"/p/b", "/p/a", "/p/a"})}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Find("ide")
	if got.Title != "Ideas" || strings.Join(got.Projects, ",") != "/p/a,/p/b" {
		t.Fatalf("meta %+v", got)
	}
	if err := s.Delete(c.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatal("still listed")
	}
}
