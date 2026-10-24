package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// enowx browser's actions go to the app serving Browsers, and its reply
// comes back; with no app, the action says so.
func TestBrowserActionsGoThroughTheApp(t *testing.T) {
	srv := startServer(t)
	cli := connect(t, srv, nil)
	info, err := cli.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cli.BrowserAct(proto.BrowserAction{Session: info.ID, Action: proto.BrowserSnapshot}); err == nil || !strings.Contains(err.Error(), "open the app") {
		t.Fatalf("with no app: %v", err)
	}
	var app *client.Client
	app = connect(t, srv, func(ev proto.Event) {
		if ev.Name != proto.EventBrowserRequest {
			return
		}
		var req proto.BrowserRequest
		if json.Unmarshal(ev.Data, &req) != nil {
			return
		}
		go func() {
			r := proto.BrowserReply{ID: req.ID, Result: proto.BrowserResult{URL: "http://localhost:3000/", Text: req.Action + " " + req.Ref}}
			if req.Ref == "missing" {
				r.Error = "no control missing"
			}
			_ = app.BrowserReply(r)
		}()
	})
	if err := app.ServeBrowsers(); err != nil {
		t.Fatal(err)
	}
	got, err := cli.BrowserAct(proto.BrowserAction{Session: info.ID, Action: proto.BrowserClick, Ref: "3"})
	if err != nil || got.Text != "click 3" || got.URL != "http://localhost:3000/" {
		t.Fatalf("click: %+v, %v", got, err)
	}
	if _, err := cli.BrowserAct(proto.BrowserAction{Session: info.ID, Action: proto.BrowserClick, Ref: "missing"}); err == nil || !strings.Contains(err.Error(), "no control missing") {
		t.Fatalf("an action that failed: %v", err)
	}
}
