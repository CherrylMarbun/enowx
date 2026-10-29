package ui

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// Settings › Cloud from off to a team's invite: the cloud turned on, the
// app signed in with a code approved elsewhere, its devices shown, a team
// made and opened, and an invite link made for it.
func TestCloudSettingsSignInAndInvite(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "https://cloud.example.com"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	acc, _ := db.GitHubAccount(ctx, 1, "ada", "Ada", "", "")
	k, _ := seal.NewDeviceKey()
	_, phoneToken, _ := db.AddDevice(ctx, acc.ID, "Ada's other laptop", api.DeviceApp, k.Public())
	other, _ := cloudclient.New(cloud.URL, phoneToken)

	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	pa, pb := net.Pipe()
	go srv.ServeConn(pa)
	app := New(Options{})
	var later []func()
	app.onMain = func(fn func()) { later = append(later, fn) }
	cl, err := client.New(pb, func(ev proto.Event) { app.cloudEvent(ev) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	app.client, app.host = cl, hosts.Host{ID: hosts.LocalID, Name: "This Mac"}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).Width(760).Height(900).Padding(24, 32).Gap(16).Background(theme.Background).Children(func() { app.cloudSection(c) })
	}, 760, 900)
	settle := func(what string, done func() bool) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(20 * time.Millisecond) {
			tt.Frame()
			fns := later
			later = nil
			for _, fn := range fns {
				fn()
			}
			if done() {
				tt.Frame()
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: never; shows %q", what, tt.Texts())
			}
		}
	}
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatalf("%v; shows %q", err, tt.Texts())
		}
	}

	settle("off", func() bool { return tt.HasText("Turn on") })
	app.cloudSet.url = cloud.URL
	click("Turn on")
	settle("on", func() bool { return tt.HasText("Sign in") && tt.HasText(cloud.URL) })
	shot(t, tt, "cloud-on.png")
	click("Sign in")
	settle("a code", func() bool { return app.cloudSet.st.Pending != nil && tt.HasText(app.cloudSet.st.Pending.UserCode) })
	shot(t, tt, "cloud-code.png")
	if err := other.ApproveDevice(ctx, app.cloudSet.st.Pending.UserCode); err != nil {
		t.Fatal(err)
	}
	settle("signed in", func() bool { return tt.HasText("Ada") && tt.HasText("Ada's other laptop") && tt.HasText("this host") })
	if !tt.HasText("This Mac, signed in as the app") {
		t.Fatalf("the device: %q", tt.Texts())
	}

	app.cloudSet.teams.newName = "enowX Labs"
	click("Make team")
	settle("the team", func() bool { return tt.HasText("enowX Labs") && tt.HasText("owner") })
	click("Team enowX Labs")
	settle("its members", func() bool { return tt.HasText("Invite as") })
	click("Make a link")
	settle("an invite", func() bool {
		return strings.HasPrefix(app.cloudSet.teams.invite.URL, "https://cloud.example.com/invite#enxi_")
	})
	shot(t, tt, "cloud-team.png")
	if !tt.HasText(app.cloudSet.teams.invite.URL) {
		t.Fatalf("the link isn't shown: %q", tt.Texts())
	}

	click("Sign out")
	settle("signed out", func() bool { return !app.cloudSet.st.SignedIn && tt.HasText("Sign in") })
	if tt.HasText("enowX Labs") {
		t.Fatal("the teams still show, signed out")
	}
}
