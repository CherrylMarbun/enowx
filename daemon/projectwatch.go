package daemon

import (
	"crypto/sha256"
	"encoding/binary"
	"io/fs"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/enowx/enowx/core/projectlinks"
	"github.com/enowx/enowx/core/transfer"
	"github.com/enowx/enowx/proto"
)

// A project synced as its files change is watched here: its folder is
// looked over every watchEvery (each file's size and time, no content),
// and once it changed and then stayed still for watchSettle, the apps are
// told (EventProjectFilesChanged), which sync it. Looking over a large
// folder takes longer: the next look waits that much more.
const (
	watchEvery  = 2 * time.Second
	watchSettle = 1500 * time.Millisecond
)

// projectWatches are the folders watched, each by the connections that
// asked; a watch stops when none wants it.
type projectWatches struct {
	mu      sync.Mutex
	folders map[string]*projectWatch
}

type projectWatch struct {
	by   map[*conn]bool
	stop chan struct{}
}

func (s *Server) watchProject(c *conn, p proto.ProjectWatchParams) error {
	dir, err := expandHome(p.Project)
	if err != nil {
		return err
	}
	w := &s.projWatches
	w.mu.Lock()
	defer w.mu.Unlock()
	pw := w.folders[dir]
	if p.Off {
		if pw != nil {
			delete(pw.by, c)
			if len(pw.by) == 0 {
				close(pw.stop)
				delete(w.folders, dir)
			}
		}
		return nil
	}
	if pw == nil {
		if w.folders == nil {
			w.folders = map[string]*projectWatch{}
		}
		pw = &projectWatch{by: map[*conn]bool{}, stop: make(chan struct{})}
		w.folders[dir] = pw
		go s.watchFolder(dir, pw.stop)
	}
	pw.by[c] = true
	return nil
}

// dropWatches stops what connection c watched, which closed.
func (w *projectWatches) drop(c *conn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for dir, pw := range w.folders {
		delete(pw.by, c)
		if len(pw.by) == 0 {
			close(pw.stop)
			delete(w.folders, dir)
		}
	}
}

func (w *projectWatches) stopAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for dir, pw := range w.folders {
		close(pw.stop)
		delete(w.folders, dir)
	}
}

func (s *Server) watchFolder(dir string, stop <-chan struct{}) {
	last := folderPrint(dir)
	var changed time.Time // when it last changed, unsettled
	for {
		start := time.Now()
		now := folderPrint(dir)
		took := time.Since(start)
		switch {
		case now != last:
			last, changed = now, time.Now()
		case !changed.IsZero() && time.Since(changed) >= watchSettle:
			changed = time.Time{}
			s.broadcast(proto.EventProjectFilesChanged, proto.ProjectRef{Path: dir})
		}
		wait := watchEvery
		if !changed.IsZero() {
			wait = watchSettle / 2
		}
		select {
		case <-stop:
			return
		case <-time.After(wait + 4*took):
		}
	}
}

// folderPrint sums up the files of dir as they are: their paths, sizes
// and times, without .git, dependencies' folders or enowX's own.
func folderPrint(dir string) [32]byte {
	h := sha256.New()
	var buf [16]byte
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && (d.Name() == ".git" || slices.Contains(transfer.Skipped, d.Name())) {
				return filepath.SkipDir
			}
			if rel, _ := filepath.Rel(dir, p); filepath.ToSlash(rel) == projectlinks.LocalDir {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		h.Write([]byte(p))
		binary.LittleEndian.PutUint64(buf[:8], uint64(info.Size()))
		binary.LittleEndian.PutUint64(buf[8:], uint64(info.ModTime().UnixNano()))
		h.Write(buf[:])
		return nil
	})
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
