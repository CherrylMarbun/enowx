package daemon

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// A feature turned off leaves the guide, a project can turn one on for
// itself, and an agent asking for one waits for the user.
func TestFeaturesShapeTheGuide(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	g, err := c.FeaturesGuide(proto.FeaturesParams{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Guide, "browser open") || strings.Contains(g.Guide, "android tap") {
		t.Fatalf("default guide: browser in, android out expected:\n%s", g.Guide)
	}
	off := false
	if _, err := c.SetFeature(proto.FeatureSetParams{ID: "browser", On: &off, Scope: "global"}); err != nil {
		t.Fatal(err)
	}
	on := true
	dir := t.TempDir()
	if _, err := c.SetFeature(proto.FeatureSetParams{ID: "android", On: &on, Scope: "project", Project: dir}); err != nil {
		t.Fatal(err)
	}
	g, _ = c.FeaturesGuide(proto.FeaturesParams{})
	if strings.Contains(g.Guide, "browser open") {
		t.Error("browser still in the guide when off")
	}
	gp, _ := c.FeaturesGuide(proto.FeaturesParams{Project: dir})
	if !gp.On["android"] || !strings.Contains(gp.Guide, "Android") {
		t.Errorf("the project's android: %v", gp.On["android"])
	}
	if _, err := c.SetFeature(proto.FeatureSetParams{ID: "notes", On: &off, Scope: "global"}); err == nil {
		t.Error("turned notes, a core feature, off")
	}

	asked := make(chan proto.FeatureRequest, 1)
	app := connect(t, srv, func(e proto.Event) {
		if e.Name == proto.EventFeaturesRequested {
			var r proto.FeatureRequest
			if jsonOK(e.Data, &r) {
				asked <- r
			}
		}
	})
	go func() {
		select {
		case r := <-asked:
			app.AnswerFeature(proto.FeatureAnswer{RequestID: r.RequestID, Allowed: true, Scope: "global"})
		case <-time.After(5 * time.Second):
		}
	}()
	a, err := c.RequestFeature(proto.FeatureRequest{ID: "traffic", Reason: "to mock the payment API"})
	if err != nil || !a.Allowed {
		t.Fatalf("answer %+v, %v", a, err)
	}
}

func jsonOK(data []byte, v any) bool { return json.Unmarshal(data, v) == nil }
