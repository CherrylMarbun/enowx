package setup

import (
	"fmt"
	"strings"
)

// Step states a setup's status file records, one line per change:
// "<step id> <state>".
const (
	StateRunning = "running"
	StateOK      = "ok"
	StateFailed  = "failed"
)

// Script is the shell that runs steps on a host, in a terminal the user
// sees (a sudo password, a sign-in code): each step in a subshell of its
// own, its state appended to statusFile, a failure going on to the next.
func Script(steps []Step, statusFile string) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n# enowX setup: see docs/setup.md.\n")
	b.WriteString(`STATUS=` + quote(statusFile) + "\n")
	b.WriteString(`mkdir -p "$(dirname "$STATUS")"; : > "$STATUS"` + "\n")
	b.WriteString(`state() { printf '%s %s\n' "$1" "$2" >> "$STATUS"; }` + "\n")
	b.WriteString(`export PATH="$HOME/.local/share/mise/shims:$HOME/.local/bin:$PATH"` + "\n")
	for i, s := range steps {
		fmt.Fprintf(&b, "\nprintf '\\n\\033[1m[%d/%d] %%s\\033[0m\\n' %s\n", i+1, len(steps), quote(s.Title))
		fmt.Fprintf(&b, "state %s %s\n", quote(s.ID), StateRunning)
		// Not as an if's condition, where the shell ignores set -e.
		fmt.Fprintf(&b, "(\nset -e\n%s\n)\nif [ $? -eq 0 ]; then state %s %s; else state %s %s; fi\n", s.Script, quote(s.ID), StateOK, quote(s.ID), StateFailed)
	}
	b.WriteString("\nprintf '\\n\\033[1menowX setup done.\\033[0m\\n'\n")
	return b.String()
}

// Status reads a status file: each step's last state, by id.
func Status(data []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if id, state, ok := strings.Cut(strings.TrimSpace(line), " "); ok {
			out[id] = state
		}
	}
	return out
}
