package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A new credential's form offers what the host's tools already keep, and
// a token the user copies after opening the platform's page; the command
// line's login comes last.
func TestVaultQuickWays(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // no tool of the real machine is asked
	gh := filepath.Join(home, ".config", "gh", "hosts.yml")
	os.MkdirAll(filepath.Dir(gh), 0o700)
	os.WriteFile(gh, []byte("github.com:\n    user: ada\n    oauth_token: gho_fakeFOUNDtoken000000000000000000000\n"), 0o600)
	var opened []string
	old := openExternal
	openExternal = func(u string) error { opened = append(opened, u); return nil }
	t.Cleanup(func() { openExternal = old })

	h := newVaultHarness(t)
	if err := h.app.vaultCreate("correct horse", false); err != nil {
		t.Fatal(err)
	}
	h.app.vaultOf().view.OpenAdd("github")
	h.settle("the login gh keeps", func() bool { return h.tt.HasText("Use github") || h.tt.HasText("Use github-ada") })
	shot(t, h.tt, "vault-quick.png")
	label := "Use github"
	if h.tt.HasText("Use github-ada") {
		label = "Use github-ada"
	}
	h.click(label)
	h.click("Add")
	h.settle("added from gh", func() bool { return h.tt.HasText("…0000") })
	cr, _ := h.app.vaultOf().data.Find("github-ada") // the name gh's login came with
	if cr == nil || cr.Fields["token"] != "gho_fakeFOUNDtoken000000000000000000000" || cr.Account != "ada" {
		t.Fatalf("from gh: %+v", cr)
	}

	// A token made on the platform's page, copied, is offered.
	h.tt.SetClipboard("something else")
	h.app.vaultOf().view.OpenAdd("github")
	h.settle("the form", func() bool { return h.tt.HasText("Open the token page") })
	h.click("Open the token page")
	if len(opened) != 1 || opened[0] != "https://github.com/settings/tokens" {
		t.Fatalf("opened %v", opened)
	}
	h.tt.SetClipboard("ghp_fakeCOPIEDtoken00000000000000000000000")
	h.tt.Advance(time.Second)
	h.settle("the copied token offered", func() bool { return h.tt.HasText("Use it") })
	h.click("Use it")
	h.click("Add")
	h.settle("added from the copy", func() bool {
		c, _ := h.app.vaultOf().data.Find("github")
		return c != nil && c.Fields["token"] == "ghp_fakeCOPIEDtoken00000000000000000000000"
	})
	// Both names are taken: a new one takes a free one.
	h.app.vaultOf().view.OpenAdd("github")
	h.settle("the form", func() bool { return h.tt.HasText("Open the token page") })
	h.click("Paste the token")
	h.tt.Type("ghp_fakeTHIRDtoken000000000000000000000000")
	h.click("Add")
	h.settle("added under a free name", func() bool {
		c, _ := h.app.vaultOf().data.Find("github-2")
		return c != nil
	})
}
