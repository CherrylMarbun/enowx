package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/core/projectlinks"
	"github.com/enowx/enowx/core/projectsync"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/core/transfer"
	"github.com/enowx/enowx/proto"
)

// bundleName is the archive entry of a sync carrying commits: a git
// bundle of the branches sent, right after the manifest.
const bundleName = "git.bundle"

// syncRefs is where a bundle's branches land before they move the
// project's own.
const syncRefs = "refs/enowx-sync/"

// projectSessions lists the sessions of project dir that a sync sees.
func (s *Server) projectSessions(dir string) []*session.Session {
	var out []*session.Session
	for _, sess := range s.sessions.List() {
		if info := sess.Info(); info.Project == dir && !sess.Meta().Queued {
			out = append(out, sess)
		}
	}
	return out
}

func sessionKey(sess *session.Session) string {
	info := sess.Info()
	return projectsync.Key(info.Created, info.Agent)
}

// syncScan says what the copy of project p.Project here has;
// p.Project may start with ~/ for the home folder.
func (s *Server) syncScan(p proto.ProjectSyncScanParams) (proto.ProjectSyncState, error) {
	dir, err := expandHome(p.Project)
	if err != nil {
		return proto.ProjectSyncState{}, err
	}
	p.Project = dir
	if info, err := os.Stat(p.Project); err != nil || !info.IsDir() {
		return proto.ProjectSyncState{}, fmt.Errorf("sync: %s isn't a folder here", p.Project)
	}
	st := proto.ProjectSyncState{Path: p.Project}
	var files []proto.SyncFile
	if p.All {
		files, err = s.allFiles(p.Project, p.Env)
	} else {
		files, err = s.syncHash.Files(p.Project, p.Env)
	}
	if err != nil {
		return st, err
	}
	st.Files = projectlinks.Filter(files, p.Include, p.Exclude)
	if p.FilesOnly {
		return st, nil
	}
	if _, err := git.TopLevel(p.Project); err == nil {
		st.Git = true
		if st.Branches, err = branchCommits(p.Project, "refs/heads/"); err != nil {
			return st, err
		}
	}
	sessions := s.projectSessions(p.Project)
	keys := map[string]string{}
	for _, sess := range sessions {
		keys[sess.ID()] = sessionKey(sess)
	}
	for _, sess := range sessions {
		info, meta := sess.Info(), sess.Meta()
		ss := proto.SyncSession{
			Key: keys[info.ID], ID: info.ID, Parent: keys[meta.Parent], Name: meta.Name, Agent: info.Agent,
			Branch: info.Branch, Worktree: info.Worktree, Running: !info.Exited,
		}
		if _, convo := s.carriedSession(sess); convo != "" {
			if f, err := s.syncHash.Conversation(convo, info.Dir); err == nil {
				ss.Conversation = strings.TrimSuffix(filepath.Base(convo), ".jsonl")
				ss.Talk, ss.Said = f.Hash, f.Size
			}
		}
		if info.Worktree && info.Dir != p.Project {
			if ss.Files, err = s.syncHash.Files(info.Dir, p.Env); err != nil {
				return st, err
			}
		}
		st.Sessions = append(st.Sessions, ss)
	}
	return st, nil
}

// branchCommits lists the refs under prefix in dir, by short name, with
// their commits.
func branchCommits(dir, prefix string) (map[string]string, error) {
	out, err := git.Run(dir, "for-each-ref", "--format=%(refname) %(objectname)", prefix)
	if err != nil {
		return nil, err
	}
	refs := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if name, commit, ok := strings.Cut(line, " "); ok {
			refs[strings.TrimPrefix(name, prefix)] = commit
		}
	}
	return refs, nil
}

// syncPack packs what p names for the other copy: the commits of the
// branches it lacks, files, and sessions, in the layout of a transfer.
func (s *Server) syncPack(p proto.ProjectSyncPackParams) (proto.TransferInfo, error) {
	if info, err := os.Stat(p.Project); err != nil || !info.IsDir() {
		return proto.TransferInfo{}, fmt.Errorf("sync: %s isn't a folder here", p.Project)
	}
	m := transfer.Manifest{Version: transfer.Version, Name: filepath.Base(p.Project), Path: p.Project}
	type carried struct {
		sess  *session.Session
		pick  proto.SyncSessionPack
		convo string
	}
	var sessions []carried
	keys := map[string]string{}
	for _, sess := range s.projectSessions(p.Project) {
		keys[sess.ID()] = sessionKey(sess)
	}
	for _, pick := range p.Sessions {
		sess, err := s.sessions.Get(pick.ID)
		if err != nil {
			return proto.TransferInfo{}, fmt.Errorf("sync: %w", err)
		}
		ms, convo := s.carriedSession(sess)
		ms.ParentKey = keys[ms.Parent]
		if !pick.New && !pick.Talk {
			convo = "" // only its worktree's files go
		}
		m.Sessions = append(m.Sessions, ms)
		sessions = append(sessions, carried{sess: sess, pick: pick, convo: convo})
	}
	slices.SortStableFunc(m.Sessions, func(a, b transfer.Session) int {
		return boolInt(a.Parent != "") - boolInt(b.Parent != "")
	})
	bundle := ""
	if len(p.Branches) > 0 {
		var err error
		if bundle, err = s.bundle(p.Project, p.Branches, p.Have); err != nil {
			return proto.TransferInfo{}, err
		}
		if bundle != "" {
			defer os.Remove(bundle)
		}
	}
	return s.writeArchive(m, func(w *transfer.Writer) error {
		if bundle != "" {
			if err := w.AddFile(bundleName, bundle); err != nil {
				return err
			}
		}
		if err := w.AddFiles(transfer.ProjectPrefix, p.Project, p.Files); err != nil {
			return err
		}
		for _, c := range sessions {
			info := c.sess.Info()
			dir := path.Join(transfer.SessionPrefix, info.ID)
			if c.pick.New || c.pick.Talk {
				if err := w.AddBytes(path.Join(dir, "history.vt"), c.sess.History()); err != nil {
					return err
				}
			}
			if c.convo != "" {
				if err := w.AddFile(path.Join(dir, "transcript.jsonl"), c.convo); err != nil {
					return err
				}
			}
			if !info.Worktree || info.Dir == p.Project {
				continue
			}
			files := c.pick.Files
			if c.pick.New {
				var err error
				if files, err = transfer.ProjectFiles(info.Dir, false); err != nil {
					return err
				}
			}
			if err := w.AddFiles(path.Join(dir, "worktree"), info.Dir, files); err != nil {
				return err
			}
		}
		return nil
	}, len(p.Files), len(sessions))
}

// bundle bundles branches of the repository at dir without the commits
// in have (those it knows), into a file to delete after; "" when the
// other side has every commit already.
func (s *Server) bundle(dir string, branches, have []string) (string, error) {
	f, err := os.CreateTemp("", "enowx-sync-*.bundle")
	if err != nil {
		return "", err
	}
	f.Close()
	args := append([]string{"bundle", "create", "-q", f.Name()}, branches...)
	var known []string
	for _, c := range have {
		if _, err := git.Run(dir, "cat-file", "-e", c+"^{commit}"); err == nil {
			known = append(known, c)
		}
	}
	if len(known) > 0 {
		args = append(append(args, "--not"), known...)
	}
	if _, err := git.Run(dir, args...); err != nil {
		os.Remove(f.Name())
		if strings.Contains(err.Error(), "empty bundle") {
			return "", nil
		}
		return "", fmt.Errorf("sync: bundle the branches: %w", err)
	}
	return f.Name(), nil
}

// insideOf is dir/rel, refusing a rel that leaves dir.
func insideOf(dir, rel string) (string, error) {
	target := filepath.Join(dir, filepath.FromSlash(rel))
	if !strings.HasPrefix(target, filepath.Clean(dir)+string(filepath.Separator)) {
		return "", errors.New("sync: " + rel + " leaves " + dir)
	}
	return target, nil
}

// allFiles lists every file of the project at dir, what .gitignore
// ignores included, but .git and dependencies' folders, hashed.
func (s *Server) allFiles(dir string, env bool) ([]proto.SyncFile, error) {
	var out []proto.SyncFile
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if p != dir && (d.Name() == ".git" || slices.Contains(transfer.Skipped, d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if !env && transfer.IsEnvFile(d.Name()) {
			return nil
		}
		f, err := s.syncHash.File(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		f.Path = filepath.ToSlash(rel)
		out = append(out, f)
		return nil
	})
	return out, err
}
