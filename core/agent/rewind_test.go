package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// fakeClaude answers keys as Claude Code 2.1.309's Rewind does, with the
// screens it shows.
type fakeClaude struct {
	mu       sync.Mutex
	msgs     []string // the list, oldest first
	state    string   // "input", "list", "menu"
	sel      int      // in the list, len(msgs) for (current)
	opt      int      // in the menu, from 1
	input    string
	restored int // the message restored before, -1 for none
	escs     int
}

func (f *fakeClaude) text() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch f.state {
	case "list":
		var b strings.Builder
		b.WriteString(" Rewind\n Restore the code and/or conversation to the point before…\n")
		for i, m := range f.msgs {
			mark := "  "
			if i == f.sel {
				mark = "❯ "
			}
			fmt.Fprintf(&b, "   %s%s\n     No code changes\n", mark, m)
		}
		if f.sel == len(f.msgs) {
			b.WriteString("   ❯ (current)\n")
		} else {
			b.WriteString("     (current)\n")
		}
		return b.String()
	case "menu":
		opts := []string{"Restore code and conversation", "Restore conversation", "Restore code", "Never mind"}
		var b strings.Builder
		for i, o := range opts {
			mark := "  "
			if i+1 == f.opt {
				mark = "❯ "
			}
			fmt.Fprintf(&b, "   %s%d. %s\n", mark, i+1, o)
		}
		return b.String()
	}
	return "❯ " + f.input + "\n  ⏵⏵ auto mode on"
}

func (f *fakeClaude) keys(k string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.state == "input" && k == keyEsc:
		if f.escs++; f.escs == 2 {
			f.state, f.sel = "list", len(f.msgs)
		}
	case f.state == "list" && k == keyUp && f.sel > 0:
		f.sel--
	case f.state == "list" && k == keyEnter:
		f.state, f.opt = "menu", 1
	case f.state == "menu" && k == keyDown:
		f.opt++
	case f.state == "menu" && k == keyEnter:
		if f.opt == 2 {
			f.restored, f.input = f.sel, f.msgs[f.sel]
		}
		f.state = "input"
	case f.state == "input" && k == keyCtrlC:
		f.input = ""
	}
}

func TestRewindClaudeFindsTheMessageAndRestoresTheConversation(t *testing.T) {
	// A command the chat doesn't show sits between the messages.
	f := &fakeClaude{msgs: []string{"Fix the login bug", "/cost", "Add a test"}, state: "input", restored: -1}
	err := Rewind(context.Background(), "claude", 2, "Fix the login bug\nin auth.go", Screen{Text: f.text, Keys: f.keys})
	if err != nil {
		t.Fatal(err)
	}
	if f.restored != 0 || f.input != "" || f.state != "input" {
		t.Fatalf("restored %d, input %q, state %s", f.restored, f.input, f.state)
	}
}
