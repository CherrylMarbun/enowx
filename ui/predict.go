package ui

import (
	"io"
	"regexp"
	"strconv"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// Typing on a far server waits a round trip for each letter to show.
// While answers take longer than predictAfter, the letters typed show at
// once, drawn over the terminal at its cursor, underlined, until the
// program's own answer replaces them: what the terminal shows stays the
// program's alone. A hidden cursor (a full-screen program drawing its
// own), anything but a letter, or a terminal whose cells can't be placed
// exactly stops predicting.

// predictAfter is the round trip past which typing is predicted.
const predictAfter = 60 * time.Millisecond

// predictor follows a session's typing and answers. Its methods are safe
// from any goroutine; the view reads it on the main thread.
type predictor struct {
	mu      sync.Mutex
	rtt     time.Duration // smoothed time from a keystroke to an answer
	sentAt  time.Time     // a keystroke waiting for its answer
	pending []guess
	cursor  [2]int // where the first guess goes: column, row
	known   bool   // cursor holds where the terminal's cursor is
	cols    int
	cellW   int // a cell, in pixels, as the terminal said
	cellH   int
	snap    func() []byte // the terminal's state, once it exists
	redraw  func()
}

type guess struct {
	r  rune
	at time.Time
}

// echoConn is a session's connection with typing predicted.
type echoConn struct {
	inner io.ReadWriteCloser
	p     *predictor
}

func (e *echoConn) Read(b []byte) (int, error) {
	n, err := e.inner.Read(b)
	if n > 0 {
		e.p.answered()
	}
	return n, err
}

func (e *echoConn) Write(b []byte) (int, error) {
	e.p.typed(b)
	return e.inner.Write(b)
}

func (e *echoConn) Close() error { return e.inner.Close() }

// ResizePixels learns the cells' size and passes it on.
func (e *echoConn) ResizePixels(cols, rows, cellWidth, cellHeight int) error {
	e.p.mu.Lock()
	e.p.cols, e.p.cellW, e.p.cellH = cols, cellWidth, cellHeight
	e.p.pending, e.p.known = nil, false
	e.p.mu.Unlock()
	if r, ok := e.inner.(interface {
		ResizePixels(cols, rows, cellWidth, cellHeight int) error
	}); ok {
		return r.ResizePixels(cols, rows, cellWidth, cellHeight)
	}
	return nil
}

func (p *predictor) typed(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if p.sentAt.IsZero() {
		p.sentAt = now
	}
	r, size := utf8.DecodeRune(b)
	switch {
	case p.rtt < predictAfter:
		p.pending = nil
		return
	case size == len(b) && r >= ' ' && r != 0x7f && unicode.IsPrint(r):
		if len(p.pending) == 0 && !p.locate() {
			return
		}
		p.pending = append(p.pending, guess{r: r, at: now})
	case (len(b) == 1 && (b[0] == 0x7f || b[0] == '\b')) && len(p.pending) > 0:
		p.pending = p.pending[:len(p.pending)-1]
	default:
		p.pending = nil // Enter, a key that moves, a paste: no guessing
	}
	p.ask()
}

// answered notes the program answered: guesses older than most of a round
// trip are its to show now, and the cursor is read again once the
// terminal took the answer in.
func (p *predictor) answered() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if !p.sentAt.IsZero() {
		sample := now.Sub(p.sentAt)
		if p.rtt == 0 {
			p.rtt = sample
		} else {
			p.rtt = (7*p.rtt + sample) / 8
		}
		p.sentAt = time.Time{}
	}
	if len(p.pending) == 0 {
		return
	}
	keep := p.pending[:0]
	for _, g := range p.pending {
		if now.Sub(g.at) < p.rtt*6/10 {
			keep = append(keep, g)
		}
	}
	p.pending = keep
	p.known = false
	// The terminal reads the answer after this returns: look again then.
	time.AfterFunc(15*time.Millisecond, func() {
		p.mu.Lock()
		if len(p.pending) > 0 && !p.known && !p.locate() {
			p.pending = nil
		}
		p.mu.Unlock()
		p.ask()
	})
	p.ask()
}

// locate reads the terminal's cursor; p.mu is held. It reports false when
// the cursor is hidden or unknown.
func (p *predictor) locate() bool {
	if p.snap == nil {
		return false
	}
	col, row, visible := cursorIn(p.snap())
	if !visible {
		return false
	}
	p.cursor, p.known = [2]int{col, row}, true
	return true
}

func (p *predictor) ask() {
	if p.redraw != nil {
		p.redraw()
	}
}

var (
	cursorAt     = regexp.MustCompile(`\x1b\[(\d+);(\d+)H`)
	cursorHidden = regexp.MustCompile(`\x1b\[\?25l`)
)

// cursorIn finds the cursor in a terminal's snapshot: the last position
// it sets, zero-based, and whether the cursor shows.
func cursorIn(snap []byte) (col, row int, visible bool) {
	all := cursorAt.FindAllSubmatch(snap, -1)
	if len(all) == 0 {
		return 0, 0, false
	}
	last := all[len(all)-1]
	r, _ := strconv.Atoi(string(last[1]))
	c, _ := strconv.Atoi(string(last[2]))
	hidden := cursorHidden.Match(snap)
	return c - 1, r - 1, !hidden
}

// guesses are what to draw: each guess's cell, in pixels of the
// terminal's grid, and the cell's size; nil when there are none.
func (p *predictor) guesses() (rs []rune, col, row, cellW, cellH, cols int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pending) == 0 || !p.known || p.cellW == 0 {
		return nil, 0, 0, 0, 0, 0
	}
	for _, g := range p.pending {
		rs = append(rs, g.r)
	}
	return rs, p.cursor[0], p.cursor[1], p.cellW, p.cellH, p.cols
}

// drawGuesses draws a session's predicted typing over its terminal, which
// starts at pad within the view of width w.
func drawGuesses(c *ui.Context, p *predictor, padX, padY, w float32) {
	if p == nil {
		return
	}
	rs, col, row, cw, ch, cols := p.guesses()
	if len(rs) == 0 {
		return
	}
	scale, ok := cellScale(cols, cw, w-2*padX)
	if !ok {
		return // the guesses would land beside the text
	}
	cellW, cellH := float32(cw)/scale, float32(ch)/scale
	term := theme.Terminal()
	for i, r := range rs {
		x, y := col+i, row
		for x >= cols {
			x, y = x-cols, y+1
		}
		ui.Row(c.Key("guess"+strconv.Itoa(i))).Absolute().Left(padX+float32(x)*cellW).Top(padY+float32(y)*cellH).
			Size(cellW, cellH).Background(term.Background).BorderWidth(0, 0, 1, 0).BorderColor(theme.Fade(term.Foreground, 0.5)).
			Center().Children(func() {
			ui.Text(c, string(r)).Font(theme.MonoFont).FontSize(cellH * 0.72).TextColor(term.Foreground)
		})
	}
	x, y := col+len(rs), row
	for x >= cols {
		x, y = x-cols, y+1
	}
	ui.Box(c.Key("guess-cursor")).Absolute().Left(padX+float32(x)*cellW).Top(padY+float32(y)*cellH).
		Size(cellW, cellH).Background(theme.Fade(term.Cursor, 0.7))
}

// cellScale finds the screen's scale from how the terminal laid its cells
// in the view: cols of cellW pixels fill inner points, short of one more.
// It reports false unless one usual scale fits.
func cellScale(cols, cellW int, inner float32) (float32, bool) {
	var found float32
	n := 0
	for _, s := range []float32{1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 3} {
		px := inner * s
		if float32(cols*cellW) <= px+0.5 && float32((cols+1)*cellW) > px+0.5 {
			found, n = s, n+1
		}
	}
	return found, n == 1
}
