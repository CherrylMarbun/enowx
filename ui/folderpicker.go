package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// folderPicker browses the connected host's folders through its daemon,
// so picking a project works the same on this device and on a server.
// It belongs to the main thread; listings arriving late are dropped by seq.
type folderPicker struct {
	open    bool
	client  *client.Client
	listing proto.FolderListing
	loaded  bool
	loading bool
	err     string
	path    string // the path bar, as typed
	hidden  bool
	seq     int

	naming  bool // the new folder row is showing
	newName string
}

// browse lists path in the picker.
func (a *App) browse(path string) {
	f := &a.addProject
	f.seq++
	seq, cl, hidden := f.seq, f.client, f.hidden
	f.loading, f.err, f.naming = true, "", false
	go func() {
		l, err := cl.ListFolders(path, hidden)
		a.win.Update(func() {
			if f.seq != seq || !f.open {
				return
			}
			f.loading = false
			if err != nil {
				f.err = strings.TrimPrefix(err.Error(), "folders: ")
				return
			}
			f.listing, f.loaded = l, true
			f.path = tildePath(l.Path, l.Home)
		})
	}()
}

func (a *App) makeFolder() {
	f := &a.addProject
	name := strings.TrimSpace(f.newName)
	if name == "" || strings.ContainsAny(name, `/\`) {
		f.err = "Type a name for the new folder, without slashes."
		return
	}
	cl, path := f.client, joinPath(f.listing.Path, name)
	go func() {
		made, err := cl.MakeFolder(path)
		a.win.Update(func() {
			if !f.open {
				return
			}
			if err != nil {
				f.err = strings.TrimPrefix(err.Error(), "folders: ")
				return
			}
			f.newName = ""
			a.browse(made)
		})
	}()
}

func (a *App) addProjectDialog(c *ui.Context) {
	f := &a.addProject
	if !f.open {
		return
	}
	ui.DialogBase(c, &f.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(620).Padding(20).Gap(12).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi).
			Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.55))
		ui.Row(c).FillWidth().Children(func() {
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				ui.Text(c, "Open a folder").FontSize(16).Bold()
				ui.Text(c, "On "+a.host.Name+". Each folder is a project; a git repository gets branches and worktrees.").FontSize(12).TextColor(colMuted)
			})
			if iconButton(c, iconClose, "Close", false).Clicked() {
				f.open = false
			}
		})
		a.folderToolbar(c, f)
		a.folderList(c, f)
		if f.err != "" {
			ui.Text(c, f.err).FontSize(12).TextColor(colError)
		}
		a.folderFooter(c, f)
	})
}

func (a *App) folderToolbar(c *ui.Context, f *folderPicker) {
	ui.Row(c).FillWidth().Gap(6).Children(func() {
		up := iconButton(c, iconArrowUp, "Up one folder", true)
		if up.Clicked() && f.listing.Parent != "" {
			a.browse(f.listing.Parent)
		}
		if iconButton(c, iconHome, "Home folder", true).Clicked() {
			a.browse("~")
		}
		in := ui.TextInput(c, &f.path).Font(monoFont).Label("Folder path").Grow(1)
		if in.Submitted() {
			a.browse(f.path)
		}
		if f.listing.Git {
			chip(c, "git", colDone, ui.RGBA(95, 211, 154, 0.12))
		}
	})
}

func (a *App) folderList(c *ui.Context, f *folderPicker) {
	ui.Scroll(c).FillWidth().Height(320).Radius(10).Background(colBackground).Border(1, colBorderHi).Children(func() {
		ui.Column(c).FillWidth().Padding(4).Gap(1).Children(func() {
			switch {
			case !f.loaded && f.loading:
				ui.Text(c, "Reading…").TextColor(colMuted).Padding(12)
				return
			case !f.loaded:
				return
			}
			if f.naming {
				a.newFolderRow(c, f)
			}
			for _, e := range f.listing.Folders {
				a.folderRow(c.Key(e.Name), f, e)
			}
			if len(f.listing.Folders) == 0 && !f.naming {
				empty := "No folders here."
				if f.listing.Files > 0 {
					empty = fmt.Sprintf("No folders here, %d %s.", f.listing.Files, plural(f.listing.Files, "file", "files"))
				}
				ui.Text(c, empty).TextColor(colMuted).Padding(12)
			}
		})
	})
}

func (a *App) folderRow(c *ui.Context, f *folderPicker, e proto.FolderEntry) {
	row := ui.ButtonBase(c).FillWidth().Gap(10).Padding(7, 10).Radius(7).Cursor(ui.CursorPointer).Label(e.Name)
	if row.Hovered() {
		row.Background(colHover)
	}
	row.Children(func() {
		svg, col := iconFolder, colTextSoft
		if e.Git {
			svg, col = iconGit, colDone
		}
		ui.Icon(c, svg).FontSize(14).TextColor(col)
		ui.Text(c, e.Name).SingleLine().Grow(1).Shrink(1)
		if e.Git {
			chip(c, "git", colDone, ui.RGBA(95, 211, 154, 0.12))
		}
		ui.Icon(c, iconChevronRight).FontSize(12).TextColor(colMuted)
	})
	if row.Clicked() {
		a.browse(joinPath(f.listing.Path, e.Name))
	}
}

func (a *App) newFolderRow(c *ui.Context, f *folderPicker) {
	ui.Row(c).FillWidth().Gap(8).Padding(4, 6).Children(func() {
		ui.Icon(c, iconFolderPlus).FontSize(14).TextColor(colAccent)
		in := ui.TextInput(c, &f.newName).Placeholder("New folder name").AutoFocus().Grow(1)
		if in.Submitted() {
			a.makeFolder()
		}
		if ghostButton(c, "Create").Clicked() {
			a.makeFolder()
		}
		if ghostButton(c, "Cancel").Clicked() {
			f.naming, f.newName = false, ""
		}
	})
}

func (a *App) folderFooter(c *ui.Context, f *folderPicker) {
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		if ui.Switch(c, &f.hidden).Label("Show hidden folders").Changed() {
			a.browse(f.listing.Path)
		}
		ui.Text(c, "Hidden folders").FontSize(12).TextColor(colTextSoft)
		ui.Box(c).Grow(1)
		if ghostButton(c, "New folder").Clicked() && f.loaded {
			f.naming = true
		}
		if ghostButton(c, "Cancel").Clicked() {
			f.open = false
		}
		name := lastPathPart(f.listing.Path)
		b := primaryButton(c, iconFolderPlus, "Open "+name, "")
		if !f.loaded || f.loading {
			b.Disabled(true).Opacity(0.5)
		} else if b.Clicked() {
			f.open = false
			if cl := a.client; cl != nil {
				go a.addProjectPath(cl, f.listing.Path)
			}
		}
	})
}

// joinPath joins a host's folder and a name with that host's separator.
func joinPath(dir, name string) string {
	sep := "/"
	if strings.Contains(dir, `\`) && !strings.Contains(dir, "/") {
		sep = `\`
	}
	return strings.TrimSuffix(dir, sep) + sep + name
}

func lastPathPart(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	if p == "" {
		return "/"
	}
	return p
}

// tildePath writes p under home with ~, as people type it.
func tildePath(p, home string) string {
	switch {
	case home == "":
		return p
	case p == home:
		return "~"
	case strings.HasPrefix(p, home+"/") || strings.HasPrefix(p, home+`\`):
		return "~" + p[len(home):]
	}
	return p
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
