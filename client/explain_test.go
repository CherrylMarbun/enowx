package client

import (
	"errors"
	"strings"
	"testing"
)

func TestExplainAnOlderDaemon(t *testing.T) {
	if got := Explain(errors.New(`checks.list: unknown method "checks.list"`)); !strings.Contains(got, "older than the app") {
		t.Fatalf("unknown method: %q", got)
	}
	if got := Explain(errors.New("disk full")); got != "disk full" {
		t.Fatalf("other error: %q", got)
	}
}
