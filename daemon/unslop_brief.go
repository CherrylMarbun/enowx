package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/enowx/enowx/core/designbrief"
	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

// An unslop's brief: what the design or page is for, written by the agent
// before its first pass, so the passes judge the design against its
// purpose and not only against what the rules read. A canvas keeps it on
// the canvas; a project in its folder (designbrief.Path), for the next
// run and the UI builder to read.

// setBrief records the brief of id's run.
func (u *unslopRuns) setBrief(id string, b *proto.DesignBrief) (proto.CanvasUnslopRun, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	run := u.runs[id]
	if run == nil {
		return proto.CanvasUnslopRun{}, false
	}
	run.Brief = b
	return copyRun(run), true
}

// hasBrief says id's run has a brief, or there's no run to need one.
func (u *unslopRuns) hasBrief(id string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	run := u.runs[id]
	return run == nil || run.Brief != nil
}

// errNoBrief is why a pass is refused before the brief: how to write it.
func errNoBrief(how string) error {
	return fmt.Errorf("unslop: write the brief before the first pass (%s), with these sections:\n\n%s", how, designbrief.Template())
}

// projectRoot is the folder of session id's project: its repository's
// top, or the session's folder outside git.
func (s *Server) projectRoot(id string) (string, error) {
	dir, err := s.sessionDir(id)
	if err != nil {
		return "", err
	}
	if top, err := git.TopLevel(dir); err == nil && top != "" {
		return top, nil
	}
	return dir, nil
}

// projectBrief is the brief session id's project keeps, nil when none.
func (s *Server) projectBrief(id string) *proto.DesignBrief {
	root, err := s.projectRoot(id)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, designbrief.Path))
	if err != nil {
		return nil
	}
	b := designbrief.Parse(string(data))
	if designbrief.Check(b) != nil {
		return nil
	}
	return &b
}

// keepProjectBrief checks brief text and keeps it in session id's
// project.
func (s *Server) keepProjectBrief(id, text string) (*proto.DesignBrief, error) {
	b := designbrief.Parse(text)
	if err := designbrief.Check(b); err != nil {
		return nil, fmt.Errorf("browser: %w", err)
	}
	root, err := s.projectRoot(id)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, designbrief.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("browser: keep the brief: %w", err)
	}
	if err := os.WriteFile(path, []byte(designbrief.Markdown(b)), 0o644); err != nil {
		return nil, fmt.Errorf("browser: keep the brief: %w", err)
	}
	return &b, nil
}
