package daemon

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

// A canvas's design is unslopped as rendered too: the app runs the lint
// in the page at its width, and the layout at a phone's; what they find
// joins what its HTML shows.
func TestCanvasUnslopAsRendered(t *testing.T) {
	srv := startServer(t)
	var app *client.Client
	var widths []int
	app = connect(t, srv, func(e proto.Event) {
		var req proto.CanvasRenderRequest
		if e.Name != proto.EventCanvasRender || json.Unmarshal(e.Data, &req) != nil {
			return
		}
		widths = append(widths, req.Width)
		text := `{"vw":1440,"stuck":[{"s":"#hero","d":"h1 “Ship”"}]}`
		if req.Script != unslop.LintScript {
			text = `{"width":390,"overflow":120,"off":["#hero"]}`
		}
		go func() { _ = app.CanvasRenderReply(proto.CanvasRenderReply{ID: req.ID, Text: text}) }()
	})
	if err := app.ServeBrowsers(); err != nil {
		t.Fatal(err)
	}
	cv, _ := app.CreateCanvas(proto.CanvasCreateParams{Title: "Ship", Kind: proto.CanvasMockup, Frames: []proto.CanvasFrame{{Name: "Desktop", Width: 1440, Height: 900}}})
	app.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: cv.Frames[0].ID, HTML: `<main id="hero"><h1>Ship</h1></main>`})
	u, err := app.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID})
	if err != nil {
		t.Fatal(err)
	}
	rules := []string{}
	for _, f := range u.Findings {
		rules = append(rules, f.Rule)
	}
	if !slices.Contains(rules, "stuck-hidden") || !slices.Contains(rules, "overflow") || u.Score == 100 {
		t.Fatalf("as rendered: %d, %v", u.Score, rules)
	}
	if !slices.Equal(widths, []int{1440, 390}) {
		t.Fatalf("rendered at %v", widths)
	}
}
