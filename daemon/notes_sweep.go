package daemon

import (
	"log/slog"
	"time"

	"github.com/enowx/enowx/proto"
)

// sweepEvery is how often the notes' files nothing names are looked for;
// the first look waits sweepAfter, for the daemon to settle.
const (
	sweepEvery = 24 * time.Hour
	sweepAfter = 5 * time.Minute
)

// sweepNoteAssets removes notes' files nothing names any more
// (notes.SweepAssets), until done closes. The sync takes the removal to
// the other hosts.
func (s *Server) sweepNoteAssets(done <-chan struct{}) {
	wait := time.NewTimer(sweepAfter)
	defer wait.Stop()
	for {
		select {
		case <-done:
			return
		case <-wait.C:
		}
		for _, sp := range append([]proto.Space{{}}, s.spaces()...) {
			st, err := s.notesIn(sp.Team)
			if err != nil {
				continue
			}
			if removed, err := st.SweepAssets(); err != nil || len(removed) > 0 {
				slog.Default().Info("notes: files nothing names removed", "team", sp.Team, "files", len(removed), "err", err)
			}
		}
		wait.Reset(sweepEvery)
	}
}
