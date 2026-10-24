package ui

import (
	"io"
	"sync"
	"time"
)

// settledResize passes a terminal's size on to its session once the size
// stopped changing for resizeSettle: a window growing or zooming goes
// through many sizes, and each would have the program (an agent's whole
// screen) redraw, a round trip away on a server. The terminal keeps its
// size until the session takes the last one.
type settledResize struct {
	io.ReadWriteCloser
	resize func(cols, rows, cellWidth, cellHeight int) error

	mu    sync.Mutex
	timer *time.Timer
	size  [4]int
}

// resizeSettle is how long a size must hold to be passed on.
const resizeSettle = 120 * time.Millisecond

// ResizePixels keeps the size, to pass on once it held.
func (s *settledResize) ResizePixels(cols, rows, cellWidth, cellHeight int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.size = [4]int{cols, rows, cellWidth, cellHeight}
	if s.timer == nil {
		s.timer = time.AfterFunc(resizeSettle, s.send)
	} else {
		s.timer.Reset(resizeSettle)
	}
	return nil
}

func (s *settledResize) send() {
	s.mu.Lock()
	size := s.size
	s.mu.Unlock()
	_ = s.resize(size[0], size[1], size[2], size[3])
}

// Close stops a size waiting to be passed on.
func (s *settledResize) Close() error {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.mu.Unlock()
	return s.ReadWriteCloser.Close()
}
