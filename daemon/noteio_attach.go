package daemon

import (
	"fmt"
	"strings"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// attachImported attaches the notes an import made to project spec: its
// folder or name on this host, or a ref or name on another (the hosts'
// catalogs).
func (s *Server) attachImported(ids []string, spec string) error {
	cats, err := s.catalogList()
	if err != nil {
		return err
	}
	here, _ := s.userdata.ID()
	var found []proto.NoteProject
	for pass := 0; pass < 2 && len(found) == 0; pass++ { // this host's first
		for _, cat := range cats {
			if (pass == 0) != (cat.Host == here) {
				continue
			}
			for _, cp := range cat.Projects {
				np := notes.CatalogProject(cat, cp)
				if np.Ref == spec || cp.Path == spec || strings.EqualFold(cp.Name, spec) {
					found = append(found, np)
				}
			}
		}
	}
	if len(found) != 1 {
		return fmt.Errorf("notes: imported, but no one project %q to attach them to (%d found)", spec, len(found))
	}
	for _, id := range ids {
		if _, err := s.notes.Attach(id, found); err != nil {
			return err
		}
	}
	return nil
}
