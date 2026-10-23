package fileview

import (
	"path"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// rowHeight is the height of a tree row; lineHeight, of a line of a file.
const (
	rowHeight  = 26
	lineHeight = 19
	indent     = 14
)

// Draw shows the tree, or the file open.
func (v *View) Draw(c *ui.Context, o Options) {
	v.follow(o)
	if v.show != "" {
		v.openFile(o, v.show)
		v.show = ""
	}
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		if v.file != "" {
			v.fileBar(c, o)
		} else {
			v.toolbar(c)
		}
		if v.err != "" {
			ui.Row(c).FillWidth().Padding(8, 14).Background(theme.Fade(theme.Error, 0.1)).Children(func() {
				ui.Text(c, v.err).FontSize(12).TextColor(theme.Error).Shrink(1)
			})
		}
		switch {
		case v.file != "":
			v.fileText(c)
		case !v.loaded:
			centered(c, "Reading the folder…")
		case len(v.rows) == 0:
			centered(c, "This folder is empty.")
		default:
			v.tree(c, o)
		}
	})
}

func centered(c *ui.Context, text string) {
	ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Children(func() {
		ui.Text(c, text).FontSize(12.5).TextColor(theme.Muted)
	})
}

// toolbar names the folder and offers to fold it or read it again.
func (v *View) toolbar(c *ui.Context) {
	ui.Row(c).Height(42).Shrink(0).FillWidth().Padding(0, 10, 0, 14).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		ui.Icon(c, icon.Folder).FontSize(13).TextColor(theme.Accent)
		name := filepath.Base(v.root)
		if v.root == "" {
			name = "Files"
		}
		ui.Text(c, name).FontSize(12.5).SingleLine().Shrink(0)
		ui.Text(c, textfmt.ShortHome(v.root)).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(1)
		ui.Spacer(c)
		if widget.IconButton(c, icon.Collapse, "Fold every folder", false).Clicked() {
			v.foldAll()
		}
		if widget.IconButton(c, icon.Sync, "Read again", false).Clicked() {
			v.reload()
		}
	})
}

// tree is the virtual list of the folders unfolded.
func (v *View) tree(c *ui.Context, o Options) {
	rows := v.rows
	v.list.Key = func(i int) any { return rows[i].path }
	ui.List(c.Key("tree"), &v.list, len(rows), func(i int) {
		v.treeRow(c, o, rows[i])
	}).Grow(1).FillWidth().MinHeight(0).Padding(4, 0)
}

// treeRow is a folder, which unfolds, or a file, which opens.
func (v *View) treeRow(c *ui.Context, o Options, r row) {
	e := r.entry
	b := ui.ButtonBase(c).FillWidth().Height(rowHeight).Padding(0, 14, 0, float32(10+r.depth*indent)).Gap(6).
		Justify(ui.Start).Cursor(ui.CursorPointer).Label(r.path)
	if b.Hovered() {
		b.Background(theme.Hover)
	}
	b.Children(func() {
		switch {
		case e.Dir && v.open[r.path]:
			ui.Icon(c, icon.ChevronDown).FontSize(11).TextColor(theme.Muted)
		case e.Dir:
			ui.Icon(c, icon.ChevronRight).FontSize(11).TextColor(theme.Muted)
		default:
			ui.Box(c).Width(11).Shrink(0)
		}
		svg := icon.File
		if e.Dir {
			svg = icon.Folder
		}
		letter, col := statusLetter(e.Status)
		name := theme.Text
		switch {
		case e.Ignored:
			name = theme.Idle
		case letter != "":
			name = col
		}
		ui.Icon(c, svg).FontSize(13).TextColor(theme.Muted)
		ui.Text(c, e.Name).FontSize(12.5).TextColor(name).SingleLine().Shrink(1)
		ui.Spacer(c)
		if letter != "" {
			ui.Text(c, letter).Font(theme.MonoFont).FontSize(11.5).TextColor(col).SingleLine().Shrink(0)
		}
	})
	if b.Clicked() {
		if e.Dir {
			v.toggle(r.path)
		} else {
			v.openFile(o, r.path)
		}
	}
}

// statusLetter is how the tree marks git's status, and its color; "" for
// none.
func statusLetter(status string) (string, ui.Color) {
	switch status {
	case "":
		return "", theme.Text
	case "A":
		return "A", theme.Done
	case "?":
		return "U", theme.Done // untracked: new to git
	case "D":
		return "D", theme.Error
	case "R":
		return "R", theme.Accent
	}
	return "M", theme.Warning
}

// fileBar heads the file open: back to the tree, its path, its size, and
// copying its path.
func (v *View) fileBar(c *ui.Context, o Options) {
	ui.Row(c).Height(42).Shrink(0).FillWidth().Padding(0, 10, 0, 8).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		if widget.IconButton(c, icon.ArrowLeft, "All files", false).Clicked() {
			v.openFile(o, "")
			return
		}
		dir, name := path.Split(v.file)
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			if dir != "" {
				ui.Text(c, dir).Font(theme.MonoFont).FontSize(12).TextColor(theme.Muted).SingleLine().Shrink(1)
			}
			ui.Text(c, name).Font(theme.MonoFont).FontSize(12).SingleLine().Shrink(0)
		})
		if v.content != nil {
			ui.Text(c, textfmt.Bytes(uint64(v.content.Size))).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(0)
		}
		if widget.IconButton(c, icon.Copy, "Copy the path", false).Clicked() {
			c.WriteClipboard(filepath.Join(v.root, filepath.FromSlash(v.file)))
		}
	})
}

// fileText is the file's lines, numbered, or why it shows none.
func (v *View) fileText(c *ui.Context) {
	f := v.content
	switch {
	case v.reading || f == nil:
		centered(c, "Reading…")
		return
	case f.Binary:
		centered(c, "A binary file: nothing to show as text.")
		return
	case len(v.lines) == 0:
		centered(c, "This file is empty.")
		return
	}
	lines := v.lines
	n := len(lines)
	if f.Truncated {
		n++
	}
	width := float32(len(strconv.Itoa(len(lines))))*7.5 + 16
	ui.List(c.Key("file:"+v.file), &v.fileList, n, func(i int) {
		if i == len(lines) {
			ui.Row(c).FillWidth().Height(lineHeight + 6).PaddingX(14).Children(func() {
				ui.Text(c, "…the rest of this file is too long to show here").FontSize(11.5).TextColor(theme.Muted).SingleLine()
			})
			return
		}
		ui.Row(c).FillWidth().Height(lineHeight).Clip().Children(func() {
			ui.Text(c, strconv.Itoa(i+1)).Font(theme.MonoFont).FontSize(11).TextColor(theme.Idle).Width(width).PaddingX(6).
				TextAlign(ui.End).SingleLine().Shrink(0)
			ui.Text(c, lines[i]).Font(theme.MonoFont).FontSize(12).TextColor(theme.TextSoft).SingleLine().Shrink(1)
		})
	}).Grow(1).FillWidth().MinHeight(0).Padding(6, 0)
}
