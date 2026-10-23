package transport

import (
	"testing"
	"time"
)

// A daemon starting while another stops waits for it to let go, then runs.
func TestLockDaemonWaitsForTheOldOne(t *testing.T) {
	dir := t.TempDir()
	first, err := LockDaemon(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan error, 1)
	go func() {
		second, err := LockDaemon(dir)
		if err == nil {
			second.Close()
		}
		got <- err
	}()
	select {
	case err := <-got:
		t.Fatalf("second daemon locked while the first ran: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	first.Close()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second daemon still waiting after the first let go")
	}
}
