package session

import (
	"strings"
)

// lineMax bounds a line kept as typed: a prompt, not a file pasted.
const lineMax = 4000

// lineTyper follows what is typed into a session, line by line, for the
// prompts the user sends an agent: Enter sends the line; a bracketed
// paste is part of it, its newlines included; escape sequences (arrows,
// focus reports) are not text. Owned by the session's mu.
type lineTyper struct {
	line    []rune
	esc     []rune // an escape sequence being read
	inEsc   bool
	pasting bool
}

// feed takes what was typed, p, and returns the lines it sent.
func (t *lineTyper) feed(p []byte) []string {
	var sent []string
	for _, r := range string(p) {
		switch {
		case t.inEsc:
			t.esc = append(t.esc, r)
			// A CSI ends with a letter or ~ past its opening [; others
			// after one rune.
			if len(t.esc) == 1 && r != '[' && r != 'O' || len(t.esc) > 1 && r >= '@' && r <= '~' {
				switch string(t.esc) {
				case "[200~":
					t.pasting = true
				case "[201~":
					t.pasting = false
				}
				t.inEsc, t.esc = false, nil
			}
		case r == 0x1b:
			t.inEsc, t.esc = true, nil
		case (r == '\r' || r == '\n') && t.pasting:
			t.line = append(t.line, '\n')
		case r == '\r' || r == '\n':
			if line := strings.TrimSpace(string(t.line)); line != "" {
				sent = append(sent, line)
			}
			t.line = nil
		case r == 0x7f || r == 0x08:
			if len(t.line) > 0 {
				t.line = t.line[:len(t.line)-1]
			}
		case r == 0x15 || r == 0x03: // Ctrl-U, Ctrl-C
			t.line = nil
		case r >= ' ' || r == '\t':
			if len(t.line) < lineMax {
				t.line = append(t.line, r)
			}
		}
	}
	return sent
}
