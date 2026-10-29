package autopush

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/core/git"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git.Run(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// repo is a repository with a commit on main, a bare remote as origin,
// and the user on a branch of theirs with work not committed.
func repo(t *testing.T) (dir, remote string) {
	t.Helper()
	root := t.TempDir()
	dir, remote = filepath.Join(root, "app"), filepath.Join(root, "remote.git")
	os.MkdirAll(dir, 0o755)
	gitT(t, root, "init", "-q", "--bare", remote)
	gitT(t, dir, "init", "-q", "-b", "main")
	gitT(t, dir, "config", "user.email", "dev@example.com")
	gitT(t, dir, "config", "user.name", "Dev")
	write(t, dir, ".gitignore", "build/\n.env\n")
	write(t, dir, "main.go", "package main\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "first")
	gitT(t, dir, "remote", "add", "origin", remote)
	gitT(t, dir, "checkout", "-q", "-b", "feature")
	write(t, dir, "main.go", "package main\n\nfunc main() {}\n") // changed, staged
	gitT(t, dir, "add", "main.go")
	write(t, dir, "notes.txt", "half done\n") // new, not staged
	write(t, dir, "build/out.bin", "ignored") // ignored
	write(t, dir, ".env", "TOKEN=x\n")        // ignored, and a secret
	write(t, dir, "deploy.pem", "not really") // a secret by its name
	return dir, remote
}

// The snapshot holds the work, not what's ignored or secret, and the
// user's branch, index and files stay as they were.
func TestSnapshotLeavesTheUsersWorkAlone(t *testing.T) {
	dir, remote := repo(t)
	head := gitT(t, dir, "rev-parse", "HEAD")
	status := gitT(t, dir, "status", "--porcelain")
	staged := gitT(t, dir, "diff", "--cached", "--name-only")

	now := time.Date(2026, 10, 26, 14, 30, 0, 0, time.UTC)
	snap, err := Take(dir, Options{Branch: DefaultBranch, Host: "Mac", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Changed || snap.Files != 2 { // main.go and notes.txt
		t.Fatalf("snapshot = %+v, want 2 files changed", snap)
	}
	if len(snap.Skipped) != 1 || snap.Skipped[0] != "deploy.pem" {
		t.Errorf("skipped = %v, want deploy.pem", snap.Skipped)
	}
	files := gitT(t, dir, "ls-tree", "-r", "--name-only", DefaultBranch)
	if files != ".gitignore\nmain.go\nnotes.txt" {
		t.Errorf("snapshot's files = %q", files)
	}
	if msg := gitT(t, dir, "log", "-1", "--format=%s", DefaultBranch); msg != "autosync: 2 files · Mac · 2026-10-26 14:30" {
		t.Errorf("message = %q", msg)
	}
	if got := gitT(t, dir, "rev-parse", "HEAD"); got != head {
		t.Error("HEAD moved")
	}
	if got := git.CurrentBranch(dir); got != "feature" {
		t.Errorf("checked out %q", got)
	}
	if got := gitT(t, dir, "status", "--porcelain"); got != status {
		t.Errorf("status = %q, was %q", got, status)
	}
	if got := gitT(t, dir, "diff", "--cached", "--name-only"); got != staged {
		t.Errorf("staged = %q, was %q", got, staged)
	}

	if err := Push(dir, "", DefaultBranch, nil); err != nil {
		t.Fatal(err)
	}
	if got := gitT(t, remote, "rev-parse", "refs/heads/"+DefaultBranch); got != snap.Commit {
		t.Errorf("remote has %s, want %s", got, snap.Commit)
	}
	if out, _ := git.Run(remote, "rev-parse", "--verify", "--quiet", "refs/heads/feature"); out != "" {
		t.Error("the user's branch was pushed")
	}

	again, err := Take(dir, Options{Branch: DefaultBranch, Host: "Mac", Now: now.Add(time.Hour)})
	if err != nil || again.Changed || again.Commit != snap.Commit {
		t.Errorf("unchanged tree: %+v, %v; want no commit", again, err)
	}
	write(t, dir, "notes.txt", "done\n")
	next, err := Take(dir, Options{Branch: DefaultBranch, Now: now.Add(2 * time.Hour)})
	if err != nil || !next.Changed || next.Files != 1 {
		t.Fatalf("next = %+v, %v", next, err)
	}
	if parent := gitT(t, dir, "rev-parse", next.Commit+"^1"); parent != snap.Commit {
		t.Errorf("next's parent = %s, want the last snapshot", parent)
	}
}

func TestSecretsRefuseTheSnapshot(t *testing.T) {
	dir, _ := repo(t)
	write(t, dir, "config.js", "const key = \"AKIAABCDEFGHIJKLMNOP\";\n")
	_, err := Take(dir, Options{})
	var se *SecretsError
	if !errors.As(err, &se) || len(se.Findings) != 1 || se.Findings[0].String() != "config.js:1: an AWS access key" {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "AKIA") {
		t.Error("the secret is in the message")
	}
	if git.BranchExists(dir, DefaultBranch) {
		t.Error("the branch was made")
	}
}

func TestBranchesNeverWritten(t *testing.T) {
	dir, _ := repo(t)
	for _, b := range []string{"main", "master", "feature", "bad..name"} {
		if _, err := Take(dir, Options{Branch: b}); err == nil {
			t.Errorf("snapshot to %s taken", b)
		}
		if err := Push(dir, "", b, nil); err == nil {
			t.Errorf("pushed %s", b)
		}
	}
}

func TestBranchLink(t *testing.T) {
	for u, want := range map[string]string{
		"git@github.com:enowx/enowx.git": "https://github.com/enowx/enowx/tree/enowx/autosync",
		"https://github.com/enowx/enowx": "https://github.com/enowx/enowx/tree/enowx/autosync",
		"https://gitlab.com/a/b.git":     "https://gitlab.com/a/b/-/tree/enowx/autosync",
		"ssh://git@example.com/x/y.git":  "",
	} {
		if got := BranchLink(u, DefaultBranch); got != want {
			t.Errorf("BranchLink(%q) = %q, want %q", u, got, want)
		}
	}
}

func TestSecretFiles(t *testing.T) {
	for p, want := range map[string]bool{
		".env": true, "api/.env.production": true, ".env.example": false, "server.key": true,
		"id_ed25519": true, "id_ed25519.pub": false, "main.go": false, "credentials.json": true,
		"keys.go": false, "prod.env": true, "release.p12": true, "cert.pfx": true, "id_rsa_work": true,
		"kubeconfig": true, "kubeconfig.yaml": true, ".npmrc": true, "secrets.yaml": true, "secrets.md": false,
		"notes.md": false, "environment.md": false, "photo.png": false, "kube.go": false,
	} {
		if got := SecretFile(p); got != want {
			t.Errorf("SecretFile(%q) = %v", p, got)
		}
	}
}

// A project with nothing beyond its last commit pushes that commit as the
// branch's start.
func TestCleanProjectPushes(t *testing.T) {
	root := t.TempDir()
	dir, remote := filepath.Join(root, "app"), filepath.Join(root, "remote.git")
	os.MkdirAll(dir, 0o755)
	gitT(t, root, "init", "-q", "--bare", remote)
	gitT(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "main.go", "package main\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "-m", "first")
	gitT(t, dir, "remote", "add", "origin", remote)
	snap, err := Take(dir, Options{Branch: "enowx/autosync", Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := Push(dir, "", "enowx/autosync", nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(gitT(t, remote, "rev-parse", "refs/heads/enowx/autosync")); got != snap.Commit {
		t.Fatalf("the remote's branch at %s, the snapshot %s", got, snap.Commit)
	}
}

// Secrets are found in their common forms, binary files too, while
// ordinary notes about them pass.
func TestScanSecrets(t *testing.T) {
	jwt := "eyJhbGciOiJSUzI1NiIsImtpZCI6IjEyMyJ9" + ".eyJzdWIiOiJhZG1pbiJ9.c2lnbmF0dXJlc2lnbmF0dXJl"
	for _, text := range []string{
		"DB_PASSWORD=Sup3rS3cretProdPassw0rd",
		"export JWT_SECRET=4f9c2a7be1d04c5e9a8b7c6d5e4f3a2b",
		"STRIPE_API_KEY: 'AbCdEfGhIjKlMnOp'",
		"users:\n- name: admin\n  user:\n    token: " + jwt,
		"    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQ==",
		"Authorization: Bearer " + jwt,
		"//registry.npmjs.org/:_authToken=" + "npm_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
		"HF_TOKEN=" + "hf_aBcDeFgHiJkLmNoPqRsTuVwXyZ01234567",
		"\x30\x82\x0a\x00\x00binary keystore\x00-----BEGIN PRIVATE KEY-----\n",
	} {
		if len(ScanSecrets("f", []byte(text))) == 0 {
			t.Errorf("no secret found in %q", text)
		}
	}
	for _, text := range []string{
		"# Deploy\n\nSet API_KEY=your-api-key-here in .env, then run make deploy.",
		"TOKEN=${GITHUB_TOKEN}",
		"PASSWORD=<password>",
		"DB_PASSWORD=changeme",
		"SECRET_NAME=prod-db-credentials-name",
		"The password is in the vault; ask Ada for the token.",
		"MAX_TOKENS=4096",
		"token: ${{ secrets.NPM_TOKEN }}",
		"The tokenizer splits text: hello world",
		"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR plain image bytes",
	} {
		if f := ScanSecrets("f", []byte(text)); len(f) > 0 {
			t.Errorf("%q: %v", text, f)
		}
	}
}
