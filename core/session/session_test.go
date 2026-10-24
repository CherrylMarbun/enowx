package session

import (
	"errors"
	"runtime"
	"testing"
)

// Two replacements of one session at once (the scheduler hibernating it as
// the user stops it): one wins, the other is refused, nothing panics.
func TestReplacingASessionTwiceAtOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	m := NewManager()
	s, err := m.Create(Spec{ID: "a", Command: []string{"sh", "-c", "sleep 30"}, Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer m.CloseAll()
	errs := make(chan error, 8)
	for range 8 {
		go func() {
			_, err := m.create(Spec{ID: "a", Exited: true, Command: s.Info().Command}, s)
			errs <- err
		}()
	}
	won := 0
	for range 8 {
		switch err := <-errs; {
		case err == nil:
			won++
		case !errors.Is(err, ErrReplaced):
			t.Fatal(err)
		}
	}
	if won != 1 {
		t.Fatalf("%d replacements won", won)
	}
}
