package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvSyncThroughTheProtocol(t *testing.T) {
	c := connect(t, startServer(t), nil)
	mac, vps := t.TempDir(), t.TempDir()
	skill := filepath.Join(mac, ".claude", "skills", "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: review\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The daemon reads its home per request; switching HOME stands in for
	// a second host.
	t.Setenv("HOME", mac)
	scan, err := c.EnvScan()
	if err != nil {
		t.Fatal(err)
	}
	var hash string
	for _, it := range scan.Items {
		if it.ID == "claude/skill/review" {
			hash = it.Hash
		}
	}
	if hash == "" {
		t.Fatalf("scan has no skill: %+v", scan.Items)
	}
	p, err := c.EnvExport("claude/skill/review")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", vps)
	if _, err := c.EnvImport(p); err != nil {
		t.Fatal(err)
	}
	after, err := c.EnvScan()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range after.Items {
		if it.ID == "claude/skill/review" && it.Hash == hash {
			return
		}
	}
	t.Fatalf("the server's scan lacks the synced skill: %+v", after.Items)
}
