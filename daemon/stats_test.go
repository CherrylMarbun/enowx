package daemon

import (
	"runtime"
	"testing"
	"time"

	"github.com/enowx/enowx/core/session"
)

func TestStatsMeasureEachSession(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("not measured on " + runtime.GOOS)
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	busy, err := srv.sessions.Create(session.Spec{Command: []string{"sh", "-c", "while :; do :; done"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	idle, err := srv.sessions.Create(session.Spec{Command: []string{"sleep", "30"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Stats(); err != nil { // the first sample has no CPU yet
		t.Fatal(err)
	}
	time.Sleep(minSample + 200*time.Millisecond)
	st, err := c.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Supported || st.Host.MemTotal == 0 || st.Daemon.RSS == 0 {
		t.Fatalf("stats = %+v", st)
	}
	got := map[string]float64{}
	for _, s := range st.Sessions {
		if s.RSS == 0 || s.Procs == 0 {
			t.Fatalf("session %s measured nothing: %+v", s.ID, s)
		}
		got[s.ID] = s.CPU
	}
	if got[busy.ID()] < 50 {
		t.Fatalf("busy session CPU = %.0f%%, want a core: %+v", got[busy.ID()], st.Sessions)
	}
	if cpu, ok := got[idle.ID()]; !ok || cpu > 10 {
		t.Fatalf("idle session CPU = %.0f%% (measured %v)", cpu, ok)
	}
}
