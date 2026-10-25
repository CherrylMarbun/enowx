package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/ui/theme"
)

func TestRunAsFieldPicksRoot(t *testing.T) {
	a := New(Options{})
	a.openEditHost(hosts.Host{ID: "x", Name: "vps", Address: "root@203.0.113.1", RunAs: "enowx"})
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(800).Height(900).Background(theme.Background).Children(func() { a.addHostDialog(c) })
	}, 800, 900)
	tt.Frame()
	if !tt.HasText("Run sessions as (when signed in as root)") {
		t.Fatalf("no field: %q", tt.Texts())
	}
	shot(t, tt, "runas-form.png")
	if err := tt.Click("root"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.addHost.asRoot {
		t.Fatal("root not picked")
	}
	shot(t, tt, "runas-form-root.png")
}
