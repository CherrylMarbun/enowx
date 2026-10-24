package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// While answers are slow, letters typed are guessed at the cursor until
// the program answers; Enter, a hidden cursor or a fast link guess nothing.
func TestTypingIsGuessedOnSlowLinks(t *testing.T) {
	snap := []byte("\x1b[Hhello\r\n$ \x1b[2;3H")
	p := &predictor{snap: func() []byte { return snap }}
	guessed := func() string {
		rs, _, _, _, _, _ := p.guesses()
		return string(rs)
	}
	p.cellW, p.cellH, p.cols = 16, 32, 80

	p.typed([]byte("l")) // fast so far: nothing guessed
	p.answered()
	if g := guessed(); g != "" || p.rtt == 0 {
		t.Fatalf("on a fast link: %q, rtt %v", g, p.rtt)
	}

	p.rtt = 200 * time.Millisecond
	for _, k := range []string{"l", "s", "x", "\x7f"} {
		p.typed([]byte(k))
	}
	if g := guessed(); g != "ls" {
		t.Fatalf("guessed %q", g)
	}
	if _, col, row, _, _, _ := p.guesses(); col != 2 || row != 1 {
		t.Fatalf("at %d,%d", col, row)
	}
	p.typed([]byte("\r"))
	if g := guessed(); g != "" {
		t.Fatalf("after Enter: %q", g)
	}

	// The program answering takes back what it had time to answer.
	p.typed([]byte("a"))
	p.mu.Lock()
	p.pending[0].at = time.Now().Add(-time.Second)
	p.mu.Unlock()
	p.typed([]byte("b"))
	p.answered()
	time.Sleep(50 * time.Millisecond)
	if g := guessed(); g != "b" {
		t.Fatalf("after an answer: %q", g)
	}

	// A full-screen program hiding the cursor draws its own: no guesses.
	p.typed([]byte("\r"))
	snap = []byte("\x1b[?25l\x1b[H$ \x1b[1;3H")
	p.typed([]byte("q"))
	if g := guessed(); g != "" {
		t.Fatalf("with the cursor hidden: %q", g)
	}
}

// The guesses show over the terminal, in its cells.
func TestGuessesShowOverTheTerminal(t *testing.T) {
	scale := 2
	p := &predictor{snap: func() []byte { return []byte("\x1b[1;3H") }, rtt: time.Second}
	p.cellW, p.cellH, p.cols = 8*scale, 16*scale, 80
	p.typed([]byte("h"))
	p.typed([]byte("i"))
	w := float32(80*8 + 2*termPadX)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(w).Height(200).Background(theme.Background).Children(func() {
			drawGuesses(c, p, termPadX, termPadY, w)
		})
	}, int(w), 200)
	tt.Frame()
	if !tt.HasText("h") || !tt.HasText("i") {
		t.Fatalf("shown: %q", tt.Texts())
	}
	// A view whose cells don't fit what the terminal said shows nothing.
	tt2 := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(300).Height(200).Children(func() { drawGuesses(c, p, termPadX, termPadY, 300) })
	}, 300, 200)
	tt2.Frame()
	if tt2.HasText("h") {
		t.Fatal("guesses drawn where the cells can't be placed")
	}
}
