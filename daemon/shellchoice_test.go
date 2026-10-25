//go:build !windows

package daemon

import (
	"os/exec"
	"slices"
	"testing"

	"github.com/enowx/enowx/proto"
)

// The shell picked for a host is what its terminals run, and a tab can
// run another of its shells; it stays picked when the daemon starts again.
func TestTerminalsStartTheShellPicked(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	t.Setenv("SHELL", sh)
	t.Cleanup(func() { chosenShell.Store(new(string)) })
	dir := t.TempDir()
	srv, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := connect(t, srv, nil)
	list, err := c.Shells()
	if err != nil || len(list.Shells) == 0 || !list.Shells[0].Login || list.Default != "" {
		t.Fatalf("shells %+v, %v", list, err)
	}
	if _, err := c.SetShell("/no/such/shell"); err == nil {
		t.Fatal("a shell the host hasn't was picked")
	}
	pick := list.Shells[len(list.Shells)-1].Path
	if _, err := c.SetShell(pick); err != nil {
		t.Fatal(err)
	}
	info, err := c.CreateSession(proto.SessionCreateParams{Agent: "shell", Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(info.Command, pick) {
		t.Fatalf("terminal runs %q, want %s", info.Command, pick)
	}
	tab, err := c.CreateSession(proto.SessionCreateParams{Agent: "shell", Dir: t.TempDir(), Shell: sh})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tab.Command, sh) {
		t.Fatalf("tab runs %q, want %s", tab.Command, sh)
	}
	srv.Close()

	chosenShell.Store(new(string))
	again, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if l, _ := connect(t, again, nil).Shells(); l.Default != pick {
		t.Fatalf("after a restart the default is %q, want %q", l.Default, pick)
	}
}
