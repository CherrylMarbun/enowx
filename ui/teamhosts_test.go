//go:build !windows

package ui

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/hostauth"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// A member opens a host a teammate shares from the rail: the app gets a
// ticket from the cloud through its own host, connects, and shows the
// shared project and its session.
func TestMemberOpensASharedHost(t *testing.T) {
	t.Setenv(proto.EnvTeamListen, "127.0.0.1:0")
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	host := func(gh int64, login string) (*client.Client, string) {
		dir := t.TempDir()
		auth := hostauth.Open(filepath.Join(dir, "cloud"))
		dk, _ := auth.DeviceKey()
		acc, _ := db.GitHubAccount(ctx, gh, login, login, "", "")
		dev, token, _ := db.AddDevice(ctx, acc.ID, login+"'s", api.DeviceHost, dk.Public())
		auth.Put(hostauth.State{URL: cloud.URL, Token: token, AccountID: acc.ID, Account: login, DeviceID: dev.ID})
		srv, err := daemon.NewServer(dir)
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
		t.Cleanup(func() { cl.Close() })
		return cl, acc.ID
	}
	ada, adaAcc := host(1, "ada")
	bob, bobAcc := host(2, "bob")
	team, _ := db.CreateTeam(ctx, adaAcc, "Design")
	inv, _ := db.CreateInvite(ctx, team.ID, adaAcc, api.InviteParams{Role: api.RoleMember})
	db.AcceptInvite(ctx, inv.Token, bobAcc)
	for _, cl := range []*client.Client{ada, bob} {
		var teams []api.Team
		if err := cl.CloudCall("GET", api.PathTeams, nil, &teams); err != nil { // the host learns its teams
			t.Fatal(err)
		}
	}
	project := t.TempDir()
	if _, err := ada.AddProject(project); err != nil {
		t.Fatal(err)
	}
	ada.CreateSession(proto.SessionCreateParams{Name: "Checkout page", Dir: project, Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}})
	if _, err := ada.SetHostShare(team.ID, []string{project}); err != nil {
		t.Fatal(err)
	}
	for _, cl := range []*client.Client{ada, bob} {
		if _, err := cl.SyncSpace(team.ID); err != nil {
			t.Fatal(err)
		}
	}

	app := New(Options{})
	var mu sync.Mutex
	var later []func()
	app.onMain = func(fn func()) { mu.Lock(); later = append(later, fn); mu.Unlock() }
	app.gen, app.client = 1, bob
	app.host = hosts.Host{ID: hosts.LocalID, Name: "bob's Mac"}
	app.state.RailWide = true
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Row(c).Width(1100).Height(700).Background(theme.Background).TextColor(theme.Text).Children(func() {
			ui.Column(c).Width(railWide).FillHeight().Children(func() { app.railTeamHosts(c, true) })
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				if app.page == pageTeamHost {
					app.teamHostPage(c)
				}
			})
		})
	}, 1100, 700)
	settle := func(what string, done func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			tt.Frame()
			if done() {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: never; shows %q", what, tt.Texts())
			}
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			fns := later
			later = nil
			mu.Unlock()
			for _, fn := range fns {
				fn()
			}
		}
	}
	settle("the shared host in the rail", func() bool { return tt.HasText("TEAM") && len(app.teamHostsOf().hosts) == 1 })
	name := app.teamHostsOf().hosts[0].Name
	if err := tt.Click(name); err != nil {
		t.Fatal(err)
	}
	settle("its shared project and session", func() bool { return tt.HasText("Checkout page") && tt.HasText("as member") })
	shot(t, tt, "team-host.png")
}
