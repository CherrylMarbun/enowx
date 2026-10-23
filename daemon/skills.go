package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/proto"
)

// Repositories fetched for their skills are cloned into <daemon dir>/fetch,
// one folder an id, until their skills are installed; a day-old one is
// left over and goes at the next start.

func (s *Server) fetchDir() string { return filepath.Join(filepath.Dir(s.WorktreeRoot), "fetch") }

// fetchSkills clones p.URL and lists its skills.
func (s *Server) fetchSkills(p proto.SkillFetchParams) (proto.SkillFetch, error) {
	id := newTransferID()
	dir := filepath.Join(s.fetchDir(), id)
	if err := os.MkdirAll(s.fetchDir(), 0o700); err != nil {
		return proto.SkillFetch{}, err
	}
	found, err := envsync.FetchSkills(p.URL, dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return proto.SkillFetch{}, err
	}
	return proto.SkillFetch{ID: id, Skills: found}, nil
}

// installSkills adds the skills picked from fetch p.ID, then forgets it.
func (s *Server) installSkills(p proto.SkillInstallParams) ([]string, error) {
	if !transferID.MatchString(p.ID) {
		return nil, errors.New("not a fetched repository")
	}
	dir := filepath.Join(s.fetchDir(), p.ID)
	defer os.RemoveAll(dir)
	root, sc := "", envsync.User
	if p.Project != "" {
		d, err := projectDir(p.Project)
		if err != nil {
			return nil, err
		}
		root, sc = d, envsync.Project
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = home
	}
	var added []string
	for _, rel := range p.Paths {
		payload, err := envsync.SkillFrom(dir, rel, p.Agent, sc)
		if err == nil {
			payload.Home = root
			_, err = envsync.ImportIn(root, payload, sc)
		}
		if err != nil {
			return added, err
		}
		added = append(added, payload.Item.Name)
	}
	return added, nil
}

// sweepFetches deletes repositories fetched and never installed from.
func (s *Server) sweepFetches() {
	entries, _ := os.ReadDir(s.fetchDir())
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > transferMaxAge {
			_ = os.RemoveAll(filepath.Join(s.fetchDir(), e.Name()))
		}
	}
}
