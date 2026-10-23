package libraryview

import (
	"path"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/widget"
)

// The library shows the user's items (under the home folder), or a
// project's own (under its folder: CLAUDE.md, .mcp.json, …): every call
// to the daemon goes to the scope shown.

func (v *View) scope() envsync.Scope {
	if v.project != "" {
		return envsync.Project
	}
	return envsync.User
}

func (v *View) fetchScan(cl *client.Client) (proto.EnvScan, error) {
	if v.project != "" {
		return cl.EnvProjectScan(v.project)
	}
	return cl.EnvScan()
}

func (v *View) fetchItem(cl *client.Client, id string) (proto.EnvPayload, error) {
	if v.project != "" {
		return cl.EnvProjectExport(v.project, id)
	}
	return cl.EnvExport(id)
}

func (v *View) writeItem(cl *client.Client, p proto.EnvPayload) (proto.EnvImportResult, error) {
	if v.project != "" {
		return cl.EnvProjectImport(v.project, p)
	}
	return cl.EnvImport(p)
}

func (v *View) deleteItem(cl *client.Client, id string) error {
	if v.project != "" {
		return cl.EnvProjectDelete(v.project, id)
	}
	return cl.EnvDelete(id)
}

// scopePicker switches between the user's items and the project's.
func (v *View) scopePicker(c *ui.Context, o Options) {
	if o.Project == "" {
		if v.project != "" {
			v.setScope(o, "")
		}
		return
	}
	cur := 0
	if v.project != "" {
		cur = 1
	}
	if i := widget.Segmented(c.Key("scope"), []string{"Global", path.Base(o.Project)}, cur, nil); i >= 0 {
		next := ""
		if i == 1 {
			next = o.Project
		}
		v.setScope(o, next)
	}
}

func (v *View) setScope(o Options, project string) {
	if project == v.project {
		return
	}
	v.project, v.selected, v.item, v.editing, v.deleting, v.loaded = project, "", nil, false, false, false
	if v.kind == kindPlugins {
		v.kind = proto.EnvSkill // plugins are the user's
	}
	v.scan = proto.EnvScan{}
	v.Reload(o)
}
