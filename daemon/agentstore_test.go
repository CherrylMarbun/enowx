package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// Two OMP sessions in one folder each keep their conversations in a folder
// of their own, and each goes on with its own when woken, not the
// folder's latest.
func TestSessionsInOneFolderResumeTheirOwnConversation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("HOME", t.TempDir())
	// An OMP that records a conversation named after its folder, then waits.
	bin := t.TempDir()
	script := `#!/bin/sh
dir=""
while [ $# -gt 0 ]; do
  [ "$1" = --session-dir ] && dir="$2"
  shift
done
[ -n "$dir" ] && id=$(basename "$dir") && printf '{"type":"session","id":"%s","cwd":"%s"}\n' "$id" "$PWD" > "$dir/s_$id.jsonl"
exec sleep 60
`
	if err := os.WriteFile(filepath.Join(bin, "omp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	srv := startServer(t)
	c := connect(t, srv, nil)
	work := t.TempDir()
	var ids []string
	for range 2 {
		info, err := c.CreateSession(proto.SessionCreateParams{Agent: "omp", Dir: work, Cols: 80, Rows: 10})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, info.ID)
	}
	for _, id := range ids {
		convo := filepath.Join(srv.dataDir, "agent-sessions", id, "s_"+id+".jsonl")
		waitFor(t, "a conversation in the session's own folder", func() bool {
			_, err := os.Stat(convo)
			return err == nil
		})
	}
	if _, err := c.HibernateSessions(proto.SessionHibernateParams{IDs: ids, Force: true}); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		waitFor(t, "hibernated", func() bool {
			list, _ := c.Sessions()
			for _, s := range list {
				if s.ID == id {
					return s.Hibernated
				}
			}
			return false
		})
		woke, err := c.WakeSession(id)
		if err != nil {
			t.Fatal(err)
		}
		cmd := strings.Join(woke.Command, " ")
		if !strings.Contains(cmd, "--resume "+id) || !strings.Contains(cmd, "--session-dir "+filepath.Join(srv.dataDir, "agent-sessions", id)) {
			t.Errorf("%s woke on another conversation or folder: %s", id, cmd)
		}
	}
}
