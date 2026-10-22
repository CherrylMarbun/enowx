package transport

import (
	"os/exec"
	"testing"
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
