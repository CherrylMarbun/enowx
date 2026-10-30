package rag

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// ragEval are questions about this repository and the file that answers
// each, for TestEval.
var ragEval = []struct{ q, file, grep string }{
	{"where does an agent's account fall back to the next one at a usage limit", "daemon/accounts_fallback.go", "fallback"},
	{"how is the Codex hook trusted so it runs without asking", "core/agent/hooks_codex.go", "trusted_hash"},
	{"where are notes synced between hosts with a three-way merge", "core/usersync", "merge"},
	{"how does the vault lend a credential's variables to a session", "core/vault/lend.go", "VaultEnv"},
	{"where is the side panel's Browser opened when a dev server starts", "ui/portoffers.go", "showBrowserSoon"},
	{"how are git commits detected to run hooks after a commit", "daemon/hooks_git.go", "rev-parse"},
	{"where are team chat messages notified with a sound", "ui/teamchatnotify.go", "notify"},
	{"how is a project packed into an archive to move it to another host", "core/transfer", "tar"},
	{"where does the daemon forward a session's listening ports", "daemon/ports.go", "listen"},
	{"how does unslop measure a canvas design before and after a pass", "unslop", "measure"},
	{"where are canvas exports to MP4 or GIF made", "canvas_unslop_video.go", "gif"},
	{"how is the protocol kept backward compatible, its methods recorded", "proto/compat_test.go", "update"},
	{"where is the virtual display for desktop apps on Linux started", "core/display", "xpra"},
	{"how does the HTTP interceptor sign TLS certificates", "core/intercept", "x509"},
	{"where is an automation's schedule parsed", "core/automation", "schedule"},
	{"how does a session hibernate when idle and come back", "hibernat", "hibernate"},
	{"where are icons searched by name and keyword", "core/iconlib", "keyword"},
	{"how does auto push commit to a branch of its own without touching the index", "core/autopush", "GIT_INDEX_FILE"},
	{"where does the app connect to a server over SSH and install the daemon", "hostconn", "ssh"},
	{"how are Discord rich presence updates sent", "core/discordrpc", "presence"},
}

// TestEval measures search on this repository with enowx-rag's own
// settings on this machine (RAG_EVAL=1): recall@5, index time and tokens,
// and what grep for a word of each question would have had an agent read.
func TestEval(t *testing.T) {
	if os.Getenv("RAG_EVAL") == "" {
		t.Skip("RAG_EVAL=1 runs it against the embedder in ~/.enowx-rag/config.yaml")
	}
	root, _ := filepath.Abs("../..")
	dir := os.Getenv("RAG_EVAL_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	s := OpenSettings(dir)
	s.Set(proto.RAGSettings{Embedder: proto.RAGFromConfig})
	e := NewEngine(dir, s)
	ctx := context.Background()
	start := time.Now()
	st, err := e.Index(ctx, root, proto.RAGProject{On: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("index: %d files, %d chunks, +%d -%d skipped %d, %d tokens, %s", st.Files, st.Chunks, st.Added, st.Removed, st.Skipped, st.Tokens, time.Since(start).Round(time.Second))

	start = time.Now()
	st, err = e.Index(ctx, root, proto.RAGProject{On: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("again, nothing changed: +%d -%d, %d tokens, %s", st.Added, st.Removed, st.Tokens, time.Since(start).Round(time.Millisecond))

	found, ragTokens, grepTokens := 0, 0, 0
	for _, c := range ragEval {
		hits, err := e.Search(ctx, root, c.q, 5, nil)
		if err != nil {
			t.Fatal(err)
		}
		at := -1
		for i, h := range hits {
			ragTokens += len(h.Snippet) / 4
			if at < 0 && strings.Contains(strings.ToLower(h.File), strings.ToLower(c.file)) {
				at = i
			}
		}
		if at >= 0 {
			found++
		}
		out, _ := exec.Command("git", "-C", root, "grep", "-il", c.grep).Output()
		files := strings.Fields(string(out))
		for _, f := range files {
			if fi, err := os.Stat(filepath.Join(root, f)); err == nil {
				grepTokens += int(fi.Size()) / 4
			}
		}
		t.Logf("%-3s %s → %s (grep: %d files)", map[bool]string{true: "ok", false: "--"}[at >= 0], c.q, firstFiles(hits), len(files))
	}
	t.Logf("recall@5 %d/%d; tokens read: rag %d, grep's files %d", found, len(ragEval), ragTokens, grepTokens)
}

func firstFiles(hits []proto.RAGHit) string {
	var out []string
	for _, h := range hits {
		out = append(out, h.File)
	}
	return strings.Join(out, ", ")
}
