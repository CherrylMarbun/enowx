package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/core/transfer"
	"github.com/enowx/enowx/proto"
)

// Archives being carried between hosts live in <daemon dir>/transfers,
// named by id, until unpacked or discarded; a day-old one is left over
// from a transfer that stopped, and goes at the next start.
const transferMaxAge = 24 * time.Hour

var transferID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func (s *Server) transferDir() string {
	return filepath.Join(filepath.Dir(s.WorktreeRoot), "transfers")
}

// transferFile is archive id's file, refusing ids that aren't one.
func (s *Server) transferFile(id string) (string, error) {
	if !transferID.MatchString(id) {
		return "", fmt.Errorf("transfer: %q isn't a transfer", id)
	}
	return filepath.Join(s.transferDir(), id+".tar.gz"), nil
}

// sweepTransfers deletes archives left over from transfers that stopped.
func (s *Server) sweepTransfers() {
	entries, _ := os.ReadDir(s.transferDir())
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > transferMaxAge {
			_ = os.Remove(filepath.Join(s.transferDir(), e.Name()))
		}
	}
}

// pack packs p's project and sessions into a new archive.
func (s *Server) pack(p proto.TransferPackParams) (proto.TransferInfo, error) {
	if info, err := os.Stat(p.Project); err != nil || !info.IsDir() {
		return proto.TransferInfo{}, fmt.Errorf("transfer: %s isn't a folder here", p.Project)
	}
	m := transfer.Manifest{Version: transfer.Version, Name: filepath.Base(p.Project), Path: p.Project, Env: p.Env}
	type carried struct {
		info    proto.SessionInfo
		history []byte
		convo   string // its transcript file
	}
	var sessions []carried
	for _, sess := range s.sessions.List() {
		info, meta := sess.Info(), sess.Meta()
		if info.Project != p.Project || meta.Queued {
			continue
		}
		picked := p.Sessions == nil || slices.Contains(p.Sessions, info.ID) ||
			meta.Parent != "" && slices.Contains(p.Sessions, meta.Parent)
		if !picked {
			continue
		}
		ms, convo := s.carriedSession(sess)
		c := carried{info: info, history: sess.History(), convo: convo}
		m.Sessions = append(m.Sessions, ms)
		sessions = append(sessions, c)
	}
	// Sessions before their tabs, which the target attaches to them.
	slices.SortStableFunc(m.Sessions, func(a, b transfer.Session) int {
		return boolInt(a.Parent != "") - boolInt(b.Parent != "")
	})
	files, err := transfer.ProjectFiles(p.Project, p.Env)
	if err != nil {
		return proto.TransferInfo{}, err
	}
	return s.writeArchive(m, func(w *transfer.Writer) error {
		if err := w.AddFiles(transfer.ProjectPrefix, p.Project, files); err != nil {
			return err
		}
		for _, c := range sessions {
			dir := path.Join(transfer.SessionPrefix, c.info.ID)
			if err := w.AddBytes(path.Join(dir, "history.vt"), c.history); err != nil {
				return err
			}
			if c.convo != "" {
				if err := w.AddFile(path.Join(dir, "transcript.jsonl"), c.convo); err != nil {
					return err
				}
			}
			if c.info.Worktree && c.info.Dir != p.Project {
				wt, err := transfer.ProjectFiles(c.info.Dir, p.Env)
				if err != nil {
					return err
				}
				if err := w.AddFiles(path.Join(dir, "worktree"), c.info.Dir, wt); err != nil {
					return err
				}
			}
		}
		return nil
	}, len(files), len(sessions))
}

// writeArchive writes a new archive of manifest m and what add adds.
func (s *Server) writeArchive(m transfer.Manifest, add func(w *transfer.Writer) error, files, sessions int) (proto.TransferInfo, error) {
	id := newTransferID()
	file, _ := s.transferFile(id)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return proto.TransferInfo{}, err
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return proto.TransferInfo{}, err
	}
	err = func() error {
		w := transfer.NewWriter(f)
		manifest, err := json.Marshal(m)
		if err != nil {
			return err
		}
		if err := w.AddBytes(transfer.ManifestName, manifest); err != nil {
			return err
		}
		if err := add(w); err != nil {
			return err
		}
		return w.Close()
	}()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(file)
		return proto.TransferInfo{}, err
	}
	st, err := os.Stat(file)
	if err != nil {
		return proto.TransferInfo{}, err
	}
	return proto.TransferInfo{ID: id, Size: st.Size(), Files: files, Sessions: sessions}, nil
}

// carriedSession is sess as an archive carries it, and its agent's
// conversation file ("" for none).
func (s *Server) carriedSession(sess *session.Session) (transfer.Session, string) {
	info, meta := sess.Info(), sess.Meta()
	ms := transfer.Session{
		ID: info.ID, Parent: meta.Parent, Name: meta.Name, Prompt: info.Prompt, Agent: info.Agent,
		Branch: info.Branch, Worktree: info.Worktree, Dir: info.Dir, Skip: info.SkipPermissions,
		Budget: info.Budget, Created: info.Created,
	}
	if info.Agent == "" || info.Agent == "custom" {
		ms.Command = info.Command
	}
	convo := ""
	if agent.Base(info.Agent) == "claude" {
		if convo = s.transcriptPath(info.ID); convo != "" {
			ms.Conversation = strings.TrimSuffix(filepath.Base(convo), ".jsonl")
		}
	}
	return ms, convo
}

// readTransfer reads a part of an archive.
func (s *Server) readTransfer(p proto.TransferReadParams) (proto.TransferPart, error) {
	file, err := s.transferFile(p.ID)
	if err != nil {
		return proto.TransferPart{}, err
	}
	f, err := os.Open(file)
	if err != nil {
		return proto.TransferPart{}, fmt.Errorf("transfer: %w", err)
	}
	defer f.Close()
	size := min(max(p.Size, 1), proto.TransferPartSize)
	buf := make([]byte, size)
	n, err := f.ReadAt(buf, p.Offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return proto.TransferPart{}, fmt.Errorf("transfer: %w", err)
	}
	return proto.TransferPart{ID: p.ID, Offset: p.Offset, Data: buf[:n]}, nil
}

// writeTransfer writes a part of an archive coming in; one at offset 0
// starts it, the others follow on from where it is.
func (s *Server) writeTransfer(p proto.TransferPart) error {
	file, err := s.transferFile(p.ID)
	if err != nil {
		return err
	}
	if len(p.Data) > proto.TransferPartSize {
		return errors.New("transfer: the part is too large")
	}
	flags := os.O_WRONLY | os.O_APPEND
	if p.Offset == 0 {
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			return err
		}
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(file, flags, 0o600)
	if err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || st.Size() != p.Offset {
		return fmt.Errorf("transfer: the part at %d doesn't follow the %d bytes received", p.Offset, st.Size())
	}
	_, err = f.Write(p.Data)
	return err
}

// discardTransfer deletes an archive.
func (s *Server) discardTransfer(id string) error {
	file, err := s.transferFile(id)
	if err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func newTransferID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
