package daemon

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A daemon started from inside an agent's terminal starts its sessions
// without that agent's markers, the user's own settings kept.
func TestSessionsDropTheParentAgentsMarkers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh")
	}
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("CLAUDE_CODE_MAX_OUTPUT_TOKENS", "4096")
	srv := startServer(t)
	cl := connect(t, srv, nil)
	dir := t.TempDir()
	_, err := cl.CreateSession(proto.SessionCreateParams{Dir: dir, Cols: 80, Rows: 10,
		Command: []string{"sh", "-c", `printf '%s|%s|%s\n' "$CLAUDECODE" "$CLAUDE_CODE_CHILD_SESSION" "$CLAUDE_CODE_MAX_OUTPUT_TOKENS" > out.txt; sleep 30`}})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(waitFile(t, filepath.Join(dir, "out.txt"))); got != "||4096" {
		t.Fatalf("the session's environment: %q", got)
	}
}
