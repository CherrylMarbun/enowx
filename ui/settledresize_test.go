package ui

import (
	"sync"
	"testing"
	"time"
)

type nopConn struct{}

func (nopConn) Read([]byte) (int, error)    { select {} }
func (nopConn) Write(p []byte) (int, error) { return len(p), nil }
func (nopConn) Close() error                { return nil }

// A window going through many sizes resizes the session once, to the last.
func TestTerminalSizeIsPassedOnOnceSettled(t *testing.T) {
	var mu sync.Mutex
	var got [][2]int
	s := &settledResize{ReadWriteCloser: nopConn{}, resize: func(cols, rows, _, _ int) error {
		mu.Lock()
		got = append(got, [2]int{cols, rows})
		mu.Unlock()
		return nil
	}}
	for i := range 30 { // an animation's frames
		_ = s.ResizePixels(80+i, 24+i/2, 8, 16)
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(3 * resizeSettle)
	_ = s.ResizePixels(200, 50, 8, 16) // later, on its own
	time.Sleep(3 * resizeSettle)
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || got[0] != [2]int{109, 38} || got[1] != [2]int{200, 50} {
		t.Fatalf("resized %v", got)
	}
}
