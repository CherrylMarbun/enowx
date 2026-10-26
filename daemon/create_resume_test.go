package daemon

import (
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A conversation resumed with a message types it once the agent is back,
// as its command line resumes and takes no prompt.
func TestResumeTypesTheMessage(t *testing.T) {
	l, err := launchFor(proto.SessionCreateParams{Agent: "claude", Resume: "conv-1", Prompt: "go on with the tests"}, chatFor{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(l.command, " "), "conv-1") {
		t.Fatalf("not resumed: %v", l.command)
	}
	if !strings.Contains(l.typed, "go on with the tests") {
		t.Fatalf("the message isn't typed: %q", l.typed)
	}
}
