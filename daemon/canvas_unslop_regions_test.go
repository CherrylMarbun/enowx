package daemon

import (
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// An unslop fanned out: an agent per region, a canvas agent of its own
// grouped apart; each changes its region only and reports its passes;
// when the last is done, the redesign's agent is told. Regions holding
// each other are refused.
func TestCanvasUnslopRegions(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	page := `<style>body{background:#111;color:#eee}</style><aside id="side"><a id="nav1">Home 🏠</a></aside><main id="main"><h1 id="title">Welcome back!!</h1><div id="cards"><p id="c1">$1,200</p></div></main>`
	cv, _ := c.CreateCanvas(proto.CanvasCreateParams{Title: "App", Kind: proto.CanvasMockup})
	c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: cv.Frames[0].ID, HTML: page})
	started, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Start: true})
	if err != nil {
		t.Fatal(err)
	}
	c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpBrief, Brief: &proto.DesignBrief{Product: "App", Users: "Its team", Tone: "Calm", FirstScreen: "The work"}})
	lead, err := c.CreateSession(proto.SessionCreateParams{Agent: "shell", Name: "lead", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	f := started.Frame
	if _, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: f, Regions: []string{"#main", "#cards"}, Session: lead.ID}); err == nil {
		t.Fatal("regions holding each other were taken")
	}
	got, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: f, Regions: []string{"#side", "#main"}, Session: lead.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Run == nil || len(got.Run.Regions) != 2 {
		t.Fatalf("the run: %+v", got.Run)
	}
	side := got.Run.Regions[0]
	list, _ := c.Sessions()
	var tab proto.SessionInfo
	for _, s := range list {
		if s.ID == side.Session {
			tab = s
		}
	}
	if tab.CanvasFor != cv.ID || tab.Group != proto.UnslopGroup+f || tab.Agent != "shell" || !strings.Contains(tab.Name, "#side") {
		t.Fatalf("the region's agent: %+v", tab)
	}

	edit := func(e proto.CanvasEdit) error {
		e.ID, e.Session = cv.ID, side.Session
		if e.Frame == "" {
			e.Frame = f
		}
		_, err := c.EditCanvas(e)
		return err
	}
	if err := edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLText, Target: "nav1", HTML: "Home"}); err != nil {
		t.Fatalf("its own element: %v", err)
	}
	if err := edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLText, Target: "title", HTML: "Overview"}); err == nil {
		t.Fatal("an element of another region was changed")
	}
	if err := edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLPatch, Target: "s1", Into: "side", HTML: `<style id="s1">#side a{gap:8px}</style>`}); err != nil {
		t.Fatalf("its own styles: %v", err)
	}
	if err := edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLSet, HTML: "<p>mine</p>"}); err == nil {
		t.Fatal("the whole design was replaced")
	}
	if err := edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLText, Target: "nav1", HTML: "x", Frame: cv.Frames[0].ID}); err == nil {
		t.Fatal("the original design was changed")
	}

	if r, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: f, Region: "side", Pass: "7"}); err != nil || r.Run.Regions[0].Pass != "07 Icons" {
		t.Fatalf("a region's pass: %+v %v", r.Run, err)
	}
	c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: f, Region: "side", Pass: "done"})
	st, _ := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Status: true})
	if !st.Run.Regions[0].Done || st.Run.Regions[1].Done {
		t.Fatalf("after #side done: %+v", st.Run.Regions)
	}
	// The last region done, the redesign's agent is told to go on.
	c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: f, Region: "main", Pass: "done"})
	waitFor(t, "the lead told", func() bool {
		screen, _ := c.Screen(lead.ID)
		return strings.Contains(screen, "All 2 region agents are done")
	})
	// Done, its lease is over: it changes nothing more.
	if err := edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLText, Target: "title", HTML: "Overview"}); err != nil {
		t.Fatalf("after its lease, a session is as any: %v", err)
	}
}
