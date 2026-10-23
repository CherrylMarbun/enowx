package agent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Screen is a running agent's terminal, as a rewind drives it: what it
// shows, and keys typed into it.
type Screen struct {
	Text func() string
	Keys func(keys string)
}

const (
	keyEsc   = "\x1b"
	keyUp    = "\x1b[A"
	keyDown  = "\x1b[B"
	keyLeft  = "\x1b[D"
	keyEnter = "\r"
	keyCtrlC = "\x03"
)

// CanRewind reports whether the chat can take the agent of preset id back
// to one of its earlier messages.
func CanRewind(id string) bool { id = Base(id); return id == "claude" || id == "codex" }

// Rewind takes the agent back to before one of the user's messages, back
// messages from the end (1 for the last), whose text is text, the way its
// user would, and leaves its input empty for the message sent in its
// place. Files the agent changed since stay as they are. The agent must be
// waiting for a message.
func Rewind(ctx context.Context, id string, back int, text string, s Screen) error {
	if back < 1 {
		return errors.New("rewind: no message to go back to")
	}
	switch Base(id) {
	case "claude":
		return rewindClaude(ctx, back, text, s)
	case "codex":
		return rewindCodex(ctx, back, s)
	}
	return fmt.Errorf("rewind: %s can't go back from the chat", id)
}

// step is how long an agent gets to redraw after a key.
const step = 150 * time.Millisecond

// rewindClaude uses Claude Code's Rewind (Esc Esc), as of 2.1.309: a list
// of the user's messages, the current point selected at the bottom, then a
// menu of what to restore; restoring the conversation puts the message
// back in the input.
func rewindClaude(ctx context.Context, back int, text string, s Screen) error {
	s.Keys(keyEsc)
	sleep(ctx, 300*time.Millisecond)
	s.Keys(keyEsc)
	if !waitFor(ctx, s, func(t string) bool { return strings.Contains(t, "Rewind") && strings.Contains(t, "(current)") }) {
		return errors.New("rewind: Claude Code didn't open its Rewind list")
	}
	want := firstWords(text)
	found := false
	// The list may hold messages the chat doesn't show (commands, say):
	// go up until the message selected is the one asked for.
	for i := 0; i < back+10 && !found; i++ {
		s.Keys(keyUp)
		sleep(ctx, step)
		found = i+1 >= back && selectedIs(s.Text(), want)
	}
	if !found {
		s.Keys(keyEsc)
		return errors.New("rewind: the message isn't in Claude Code's Rewind list")
	}
	s.Keys(keyEnter)
	var n int
	if !waitFor(ctx, s, func(t string) bool { n = restoreOption(t); return n > 0 }) {
		s.Keys(keyEsc)
		return errors.New("rewind: Claude Code didn't offer to restore the conversation")
	}
	for range n - 1 {
		s.Keys(keyDown)
		sleep(ctx, step)
	}
	s.Keys(keyEnter)
	if !waitFor(ctx, s, func(t string) bool { return restoreOption(t) == 0 && !strings.Contains(t, "(current)") }) {
		return errors.New("rewind: Claude Code didn't restore the conversation")
	}
	sleep(ctx, 300*time.Millisecond)
	s.Keys(keyCtrlC) // the message it put back in the input
	return nil
}

var restoreLine = regexp.MustCompile(`(\d+)\.\s+Restore conversation\s*$`)

// restoreOption is the number of "Restore conversation" in Claude Code's
// Rewind menu on screen, 0 when the menu isn't there.
func restoreOption(screen string) int {
	for line := range strings.SplitSeq(screen, "\n") {
		if m := restoreLine.FindStringSubmatch(strings.TrimRight(line, " ")); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n
		}
	}
	return 0
}

// selectedIs reports whether the line Claude Code's Rewind list selects
// (marked ❯) starts as want.
func selectedIs(screen, want string) bool {
	lines := strings.Split(screen, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if _, after, ok := strings.Cut(lines[i], "❯ "); ok {
			return strings.HasPrefix(strings.TrimSpace(after), want)
		}
	}
	return false
}

// firstWords is the start of a message as a list shows it, on one line.
func firstWords(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if r := []rune(strings.TrimSpace(line)); len(r) > 24 {
		line = string(r[:24])
	}
	return strings.TrimSpace(line)
}

// rewindCodex uses Codex's backtrack, as of 0.164: Esc twice with the
// input empty browses the transcript at the last message, Left steps to
// earlier ones, Enter forks the conversation before it and puts it back in
// the input.
func rewindCodex(ctx context.Context, back int, s Screen) error {
	s.Keys(keyEsc)
	if !waitFor(ctx, s, func(t string) bool { return strings.Contains(t, "esc again to edit") }) {
		s.Keys(keyCtrlC)
		return errors.New("rewind: Codex didn't offer to edit a previous message; is its input empty?")
	}
	s.Keys(keyEsc)
	if !waitFor(ctx, s, func(t string) bool { return strings.Contains(t, "↵ rewind") }) {
		return errors.New("rewind: Codex didn't open its transcript")
	}
	for range back - 1 {
		s.Keys(keyLeft)
		sleep(ctx, step)
	}
	s.Keys(keyEnter)
	if !waitFor(ctx, s, func(t string) bool { return !strings.Contains(t, "↵ rewind") }) {
		return errors.New("rewind: Codex didn't go back")
	}
	sleep(ctx, 300*time.Millisecond)
	s.Keys(keyCtrlC) // the message it put back in the input
	return nil
}

// waitFor polls the screen until ok says so, for up to three seconds.
func waitFor(ctx context.Context, s Screen, ok func(screen string) bool) bool {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		if ok(s.Text()) {
			return true
		}
		sleep(ctx, 50*time.Millisecond)
	}
	return false
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
