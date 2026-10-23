package daemon

import (
	"testing"

	"github.com/enowx/enowx/proto"
)

// A tab runs in its parent's folder and is deleted with it.
func TestTabFollowsParent(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	dir := t.TempDir()
	parent, err := c.CreateSession(proto.SessionCreateParams{Command: catCommand(), Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	tab, err := c.CreateSession(proto.SessionCreateParams{Command: catCommand(), Parent: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if tab.Parent != parent.ID || tab.Dir != parent.Dir {
		t.Fatalf("tab: parent %q dir %q, want %q %q", tab.Parent, tab.Dir, parent.ID, parent.Dir)
	}
	if err := c.KillSession(parent.ID); err != nil {
		t.Fatal(err)
	}
	list, err := c.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("after deleting the parent: %d sessions left", len(list))
	}
}
