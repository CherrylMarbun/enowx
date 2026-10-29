package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/core/autopush"
	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/proto"
)

// Notes and canvases going from one space to another (docs/cloud.md,
// Personal and team): to a team's only as a member or above, after a scan
// for secrets; out of a team's only when its admins let members copy its
// items out. A note goes with the pages under it and their files.

// mayMoveSpace says whether an item may go from space from to space to
// ("" the user's own).
func (s *Server) mayMoveSpace(from, to string) error {
	if from == to {
		return errors.New("spaces: it is in that space already")
	}
	if to != "" {
		sp, ok := s.spaceByTeam(to)
		if !ok {
			return fmt.Errorf("spaces: team %s isn't on this host", to)
		}
		if sp.Role == api.RoleViewer {
			return fmt.Errorf("spaces: as a viewer of %s you can't add to it", sp.Name)
		}
	}
	if from == "" {
		return nil
	}
	sp, ok := s.spaceByTeam(from)
	if !ok {
		return fmt.Errorf("spaces: team %s isn't on this host", from)
	}
	if sp.Role == api.RoleViewer {
		return fmt.Errorf("spaces: as a viewer of %s you can't take its items", sp.Name)
	}
	st, err := s.cloud.store.Get()
	if err != nil {
		return err
	}
	c, err := cloudclient.New(st.URL, st.Token)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudTimeout)
	defer cancel()
	t, err := c.Team(ctx, from)
	if err != nil {
		return err
	}
	if !t.CopyOut {
		return fmt.Errorf("spaces: %s's admins keep its notes and canvases in the team: they let members copy them out in the team's settings", sp.Name)
	}
	return nil
}

func (s *Server) spaceByTeam(team string) (proto.Space, bool) {
	for _, sp := range s.spaces() {
		if sp.Team == team {
			return sp, true
		}
	}
	return proto.Space{}, false
}

// secretsIn says which of texts (by name) look like they hold a secret.
func secretsIn(texts map[string][]byte) error {
	var found []string
	for name, data := range texts {
		if f := autopush.ScanSecrets(name, data); len(f) > 0 {
			found = append(found, name+": "+f[0].Kind)
		}
	}
	if len(found) > 0 {
		return fmt.Errorf("spaces: not shared, it looks like it holds a secret (%s): remove it first", strings.Join(found, "; "))
	}
	return nil
}

// moveNoteSpace takes note p.ID and the pages under it to p.Team's space.
func (s *Server) moveNoteSpace(p proto.SpaceMoveParams) (proto.NoteInfo, error) {
	from := s.notesFor(p.ID)
	fromTeam := s.spaceOf("notes", p.ID+".md")
	if err := s.mayMoveSpace(fromTeam, p.Team); err != nil {
		return proto.NoteInfo{}, err
	}
	to, err := s.notesIn(p.Team)
	if err != nil {
		return proto.NoteInfo{}, err
	}
	list, err := from.List()
	if err != nil {
		return proto.NoteInfo{}, err
	}
	// The note, then the pages under it, each after its parent.
	order := []string{p.ID}
	for i := 0; i < len(order); i++ {
		for _, n := range list {
			if n.Parent == order[i] {
				order = append(order, n.ID)
			}
		}
	}
	pages := make([]proto.Note, 0, len(order))
	texts := map[string][]byte{}
	for _, id := range order {
		n, err := from.Get(id)
		if err != nil {
			return proto.NoteInfo{}, err
		}
		pages = append(pages, n)
		texts[n.Title] = []byte(n.Title + "\n" + n.Body)
	}
	if p.Team != "" {
		if err := secretsIn(texts); err != nil {
			return proto.NoteInfo{}, err
		}
	}
	ids := map[string]string{} // the pages' IDs where they went
	var root proto.NoteInfo
	for i, n := range pages {
		old := n.ID
		if p.Copy {
			n.ID = ""
		}
		n.Team = ""
		if i == 0 {
			n.Parent, n.Order = "", -1 // last at the top of where it goes
		} else {
			n.Parent = ids[n.Parent]
		}
		info, err := to.Restore(n)
		if err != nil {
			return proto.NoteInfo{}, err
		}
		ids[old] = info.ID
		if i == 0 {
			root = info
		}
	}
	// Their files, the bodies naming them where they are now.
	for _, n := range pages {
		id := ids[n.ID]
		entries, _ := os.ReadDir(from.AssetsDir(n.ID))
		for _, e := range entries {
			data, err := os.ReadFile(filepath.Join(from.AssetsDir(n.ID), e.Name()))
			if err != nil || e.IsDir() {
				continue
			}
			if _, err := to.SaveAsset(id, e.Name(), data); err != nil {
				return proto.NoteInfo{}, err
			}
		}
		if id != n.ID {
			moved, err := to.Get(id)
			if err == nil && strings.Contains(moved.Body, "assets/"+n.ID+"/") {
				moved.Body = strings.ReplaceAll(moved.Body, "assets/"+n.ID+"/", "assets/"+id+"/")
				_, _ = to.Replace(moved)
			}
		}
	}
	if !p.Copy {
		if err := from.Delete(p.ID); err != nil {
			return proto.NoteInfo{}, err
		}
	}
	s.groupsChanged(nil) // and the notes
	root.Team = p.Team
	return root, nil
}

// moveCanvasSpace takes canvas p.ID to p.Team's space: its folder, its
// agent's conversation staying behind when copied.
func (s *Server) moveCanvasSpace(p proto.SpaceMoveParams) (proto.CanvasInfo, error) {
	from := s.canvasFor(p.ID)
	fromTeam := s.teamOfCanvas(p.ID)
	if err := s.mayMoveSpace(fromTeam, p.Team); err != nil {
		return proto.CanvasInfo{}, err
	}
	to, err := s.canvasesIn(p.Team)
	if err != nil {
		return proto.CanvasInfo{}, err
	}
	src, err := from.Dir(p.ID)
	if err != nil {
		return proto.CanvasInfo{}, err
	}
	if _, err := from.Get(p.ID); err != nil {
		return proto.CanvasInfo{}, err
	}
	files := map[string][]byte{}
	err = filepath.WalkDir(src, func(full string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "chat.json" {
			return err
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, full)
		files[rel] = data
		return nil
	})
	if err != nil {
		return proto.CanvasInfo{}, err
	}
	if p.Team != "" {
		if err := secretsIn(files); err != nil {
			return proto.CanvasInfo{}, err
		}
	}
	id := p.ID
	if p.Copy {
		id = canvas.NewID()
	}
	dst, err := to.Dir(id)
	if err != nil {
		return proto.CanvasInfo{}, err
	}
	if _, err := os.Stat(dst); err == nil {
		return proto.CanvasInfo{}, fmt.Errorf("spaces: a canvas %s is there already", id)
	}
	for rel, data := range files {
		if id != p.ID && strings.HasSuffix(rel, ".json") {
			data = []byte(strings.ReplaceAll(string(data), `"`+p.ID+`"`, `"`+id+`"`))
		}
		if err := writeTeamFile(filepath.Join(dst, rel), data); err != nil {
			os.RemoveAll(dst)
			return proto.CanvasInfo{}, err
		}
	}
	if !p.Copy {
		if err := from.Delete(p.ID); err != nil {
			return proto.CanvasInfo{}, err
		}
	}
	s.canvasesChanged()
	c, err := to.Get(id)
	if err != nil {
		return proto.CanvasInfo{}, err
	}
	return s.withState(c).CanvasInfo, nil
}
