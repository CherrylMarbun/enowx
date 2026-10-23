package daemon

import (
	"runtime"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A variable set for the host reaches every session started after, the
// list masks it, and the next daemon keeps it.
func TestSessionVariables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	srv, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := connect(t, srv, nil)
	list, err := c.SetSessionEnv("API_KEY", "sk-secret-1234")
	if err != nil || len(list) != 1 || list[0].Value != "••••1234" {
		t.Fatalf("list %+v, %v", list, err)
	}
	if _, err := c.SetSessionEnv("PATH", "/x"); err == nil {
		t.Fatal("replaced PATH")
	}
	info, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", "echo key=$API_KEY; sleep 30"}, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	watch(st).waitFor(t, "key=sk-secret-1234")
	srv.Close()

	again, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if list := again.vars.list(); len(list) != 1 || list[0].Name != "API_KEY" {
		t.Fatalf("after a restart %+v", list)
	}
}
