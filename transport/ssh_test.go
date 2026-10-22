package transport

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestAsUserQuotesForTheShell(t *testing.T) {
	cmd := `echo "$HOME" 'it'"'"'s'`
	if got, want := AsUser("enowx", cmd), "su - enowx -c "+shellQuote(cmd); got != want {
		t.Fatalf("AsUser = %s, want %s", got, want)
	}
	// The quoted command must reach su's shell unchanged.
	out, err := exec.Command("sh", "-c", "printf %s "+shellQuote(cmd)).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != cmd {
		t.Fatalf("round trip = %s, want %s", out, cmd)
	}
	if AsUser("", cmd) != cmd {
		t.Fatal("AsUser without a user changed the command")
	}
}

func TestKnownAlgorithmsFollowKnownHosts(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize("100.64.0.2:22")}, key)
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	known, err := knownhosts.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := knownAlgorithms(known, "100.64.0.2:22"); !slices.Equal(got, []string{ssh.KeyAlgoED25519}) {
		t.Fatalf("known host: %v, want only %s", got, ssh.KeyAlgoED25519)
	}
	if got := knownAlgorithms(known, "100.64.0.9:22"); got != nil {
		t.Fatalf("unknown host: %v, want nil so the server picks", got)
	}
}
