package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// An agent pointed at a model API through the daemon: the key comes back
// masked, a change keeping the key keeps it, its sessions get what it
// reads from the environment, and the original settings come back.
func TestAgentAPIThroughTheDaemon(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	original := "approval_policy = \"never\"\n"
	os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(original), 0o644)
	srv := startServer(t)
	c := connect(t, srv, nil)

	info, err := c.SetAgentAPI("codex", proto.AgentAPI{BaseURL: "https://gw.example.com/v1", APIKey: "sk-secret-1234", Model: "gw-model"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Current == nil || info.Current.APIKey != "••••1234" || info.Current.Model != "gw-model" || len(info.Backups) != 1 || !info.Backups[0].Original {
		t.Fatalf("set: %+v", info)
	}
	info, err = c.SetAgentAPI("codex", proto.AgentAPI{BaseURL: "https://gw2.example.com/v1", APIKey: proto.AgentAPIKeepKey, Model: "other"})
	if err != nil || info.Current.BaseURL != "https://gw2.example.com/v1" || info.Current.APIKey != "••••1234" {
		t.Fatalf("kept key: %+v %v", info.Current, err)
	}
	l, err := launchFor(proto.SessionCreateParams{Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(l.env, "ENOWX_CODEX_API_KEY=sk-secret-1234") {
		t.Fatalf("codex sessions' env: %q", l.env)
	}
	list, err := c.AgentAPIs()
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := json.Marshal(list); strings.Contains(string(b), "sk-secret") {
		t.Fatalf("a key left the host: %s", b)
	}
	if _, err := c.RestoreAgentAPI("codex", ""); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); string(data) != original {
		t.Fatalf("restored:\n%s", data)
	}
	if l, _ := launchFor(proto.SessionCreateParams{Agent: "codex"}); slices.ContainsFunc(l.env, func(e string) bool { return strings.HasPrefix(e, "ENOWX_CODEX_API_KEY=") }) {
		t.Fatalf("a key after restoring: %q", l.env)
	}
}
