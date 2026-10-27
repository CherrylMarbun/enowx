package daemon

import (
	"log/slog"
	"time"
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
		if removed, err := s.notes.SweepAssets(); err != nil || len(removed) > 0 {
			slog.Default().Info("notes: files nothing names removed", "files", len(removed), "err", err)
		}
		wait.Reset(sweepEvery)
	}
}
