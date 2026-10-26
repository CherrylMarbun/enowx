package ui

import (
	"fmt"
	"testing"
	"time"

	mygoui "github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/proto"
)

func floatingNavigate(t *testing.T, tt *mygoui.Tester, labels ...string) {
	t.Helper()
	for _, label := range labels {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Advance(time.Second)
	}
}

func TestFloatingProjectsFilterAndBack(t *testing.T) {
	app := New(Options{})
	app.host = LocalHost()
	app.projects = []proto.ProjectInfo{{Path: "/api", Name: "API"}, {Path: "/web", Name: "Web"}, {Path: "/empty", Name: "Empty"}}
	app.sessions = []proto.SessionInfo{
		{ID: "api", Project: "/api", Name: "API task"},
		{ID: "web", Project: "/web", Name: "Web task"},
	}
	tt := floatingTester(t, app)
	floatingNavigate(t, tt, "Active sessions", "This PC")
	if !tt.HasText("API") || !tt.HasText("Web") || tt.HasText("API task") {
		t.Fatalf("expected project choices before sessions: %q", tt.Texts())
	}
	floatingNavigate(t, tt, "API")
	if !tt.HasText("API task") || tt.HasText("Web task") {
		t.Fatalf("project selection did not filter sessions: %q", tt.Texts())
	}
	tt.Key(0, mygoui.KeyEscape)
	tt.Advance(time.Second)
	if !tt.HasText("Web") || !app.floatingShown {
		t.Fatal("Escape did not return to projects")
	}
	floatingNavigate(t, tt, "Empty")
	if !tt.HasText(floatingEmptyNone) {
		t.Fatal("empty project did not report no active sessions")
	}
	floatingNavigate(t, tt, "Back", "Web", "Web task")
	if app.selected != "web" {
		t.Fatal("project session did not open")
	}
}

func TestFloatingChooseBetweenServers(t *testing.T) {
	app := New(Options{})
	app.host = LocalHost()
	app.hostList = []hosts.Host{{ID: "one", Name: "Server One"}, {ID: "two", Name: "Server Two"}}
	app.watches = map[string]*hostWatch{}
	for _, host := range app.hostList {
		app.watches[host.ID] = &hostWatch{host: host, status: hostStatus{
			projects: []proto.ProjectInfo{{Path: "/app", Name: "App"}, {Path: "/empty", Name: "Empty"}},
			sessions: []proto.SessionInfo{{ID: host.ID, Project: "/app", Name: host.Name + " task"}},
		}}
	}
	tt := floatingTester(t, app)
	floatingNavigate(t, tt, "Active sessions", "VPS")
	if !tt.HasText("Server One") || !tt.HasText("Server Two") {
		t.Fatal("VPS did not offer both servers")
	}
	floatingNavigate(t, tt, "Server Two", "App")
	if !tt.HasText("Server Two task") || tt.HasText("Server One task") {
		t.Fatalf("same project path on two hosts leaked sessions: %q", tt.Texts())
	}
	floatingNavigate(t, tt, "Back", "Back", "Server One", "Empty")
	if !tt.HasText(floatingEmptyNone) {
		t.Fatal("watched host's empty project was not selectable")
	}
	floatingNavigate(t, tt, "Back", "App")
	if !tt.HasText("Server One task") {
		t.Fatal("switching servers did not update sessions")
	}
}

func TestFloatingProjectPagesKeepAllChoices(t *testing.T) {
	app := New(Options{})
	app.host = LocalHost()
	for i := 0; i < 12; i++ {
		app.projects = append(app.projects, proto.ProjectInfo{Path: fmt.Sprintf("/p%d", i), Name: fmt.Sprintf("Project %d", i)})
	}
	tt := floatingTester(t, app)
	floatingNavigate(t, tt, "Active sessions", "This PC", "Next page", "Next page")
	if !tt.HasText("Project 11") || tt.HasText("Next page") {
		t.Fatal("last projects were not reachable")
	}
	floatingNavigate(t, tt, "Project 11", "Back")
	if !tt.HasText("Project 11") {
		t.Fatal("Back lost the selected project's page")
	}
	floatingNavigate(t, tt, "Previous page")
	if !tt.HasText("Project 5") {
		t.Fatal("previous project page did not open")
	}
}
