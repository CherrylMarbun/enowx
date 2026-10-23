package diffview

import (
	"path"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// rowHeight is the height of a diff line: the list's rows all share it,
// so a long diff scrolls without measuring every line.
const rowHeight = 20

// fileRowHeight is the height of a file in the list of changes.
const fileRowHeight = 27

// Kinds of row.
const (
	rowSection = iota // "CHANGES 12"
	rowFile           // a changed file, in the list
	rowHunk           // "@@ -1,3 +1,4 @@"
	rowLine           // a line of a hunk
	rowBinary         // a file shown without its lines
	rowCut            // the file goes on past what the daemon sent
	rowComment        // a review comment, under its line
	rowDraft          // the comment being written
)

// row is a row of the view's list: the changed files, or the lines of the
// file open.
type row struct {
	kind    int
	file    int // index in the diff's files
	hunk    int
	line    int
	comment int // index in the comments
}

// setDiff shows d: the list of files, or the file open if d still has it.
func (v *View) setDiff(d proto.Diff) {
	v.diff = d
	v.layoutRows()
}

// openFile shows the lines of the file at path; "" goes back to the list.
func (v *View) openFile(path string) {
	if path != "" && path != v.file {
		v.fileList.ScrollTo(0, ui.Start)
	}
	v.file = path
	v.layoutRows()
}

// layoutRows lays the rows out again. It makes a new slice: the list may
// still be drawing the old one this frame.
func (v *View) layoutRows() {
	v.rows = nil
	fi := v.fileIndex(v.file)
	if fi < 0 {
		v.file = ""
		v.rows = append(v.rows, row{kind: rowSection})
		for i := range v.diff.Files {
			v.rows = append(v.rows, row{kind: rowFile, file: i})
		}
		return
	}
	f := v.diff.Files[fi]
	if f.Binary || len(f.Hunks) == 0 {
		v.rows = append(v.rows, row{kind: rowBinary, file: fi})
		return
	}
	for hi, h := range f.Hunks {
		v.rows = append(v.rows, row{kind: rowHunk, file: fi, hunk: hi})
		for li, l := range h.Lines {
			v.rows = append(v.rows, row{kind: rowLine, file: fi, hunk: hi, line: li})
			for ci, c := range v.comments {
				if c.at(f.Path, l) {
					v.rows = append(v.rows, row{kind: rowComment, file: fi, comment: ci})
				}
			}
			if d := v.draft; d != nil && d.at(f.Path, l) {
				v.rows = append(v.rows, row{kind: rowDraft, file: fi, hunk: hi, line: li})
			}
		}
	}
	if f.Truncated {
		v.rows = append(v.rows, row{kind: rowCut, file: fi})
	}
}

// fileIndex is the index of the file at path in the diff, or -1.
func (v *View) fileIndex(path string) int {
	if path == "" {
		return -1
	}
	for i, f := range v.diff.Files {
		if f.Path == path {
			return i
		}
	}
	return -1
}

// lines is the virtual list of the rows.
func (v *View) lines(c *ui.Context) {
	rows, list := v.rows, &v.list
	if v.file != "" {
		list = &v.fileList
	}
	list.Key = func(i int) any { return rows[i] }
	ui.List(c.Key(v.file), list, len(rows), func(i int) {
		r := rows[i]
		switch r.kind {
		case rowSection:
			ui.Row(c).FillWidth().Height(30).PaddingX(14).Gap(6).Children(func() {
				ui.Text(c, "CHANGES").FontSize(10.5).TextColor(theme.Muted).SingleLine()
				ui.Text(c, strconv.Itoa(len(v.diff.Files))).FontSize(10.5).TextColor(theme.Idle).SingleLine()
			})
		case rowFile:
			v.fileItem(c, v.diff.Files[r.file])
		case rowHunk:
			ui.Row(c).FillWidth().Height(rowHeight).PaddingX(12).Background(theme.Sunken).Children(func() {
				ui.Text(c, v.diff.Files[r.file].Hunks[r.hunk].Header).Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(1)
			})
		case rowLine:
			v.diffLine(c, v.diff.Files[r.file].Path, v.diff.Files[r.file].Hunks[r.hunk].Lines[r.line])
		case rowComment:
			v.commentRow(c, r.comment)
		case rowDraft:
			v.draftRow(c)
		case rowBinary:
			note := "Binary file"
			if !v.diff.Files[r.file].Binary {
				note = "No lines to show"
			}
			ui.Row(c).FillWidth().Height(32).PaddingX(14).Children(func() {
				ui.Text(c, note).FontSize(11.5).TextColor(theme.Muted).SingleLine()
			})
		case rowCut:
			ui.Row(c).FillWidth().Height(rowHeight).PaddingX(12).Children(func() {
				ui.Text(c, "…the rest of this file is too long to show here").FontSize(11.5).TextColor(theme.Muted).SingleLine()
			})
		}
	}).Grow(1).FillWidth().MinHeight(0)
}

// fileItem is a changed file in the list: its status letter, its folder
// dimmed before its name, and the lines it added and removed. Clicking it
// opens its lines.
func (v *View) fileItem(c *ui.Context, f proto.DiffFile) {
	b := ui.ButtonBase(c).FillWidth().Height(fileRowHeight).PaddingX(14).Gap(10).Justify(ui.Start).
		Cursor(ui.CursorPointer).Label(f.Path)
	if b.Hovered() {
		b.Background(theme.Hover)
	}
	b.Children(func() {
		letter, col := statusLetter(f.Status)
		ui.Text(c, letter).Font(theme.MonoFont).FontSize(12).TextColor(col).Width(10).SingleLine().Shrink(0)
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			dir, name := path.Split(f.Path)
			if dir != "" {
				ui.Text(c, dir).Font(theme.MonoFont).FontSize(12).TextColor(theme.Muted).SingleLine().Shrink(1)
			}
			ui.Text(c, name).Font(theme.MonoFont).FontSize(12).TextColor(theme.Text).SingleLine().Shrink(0)
		})
		if n := v.commentsOn(f.Path); n > 0 {
			ui.Icon(c, icon.Chat).FontSize(11).TextColor(theme.Accent)
			ui.Text(c, strconv.Itoa(n)).FontSize(11).TextColor(theme.Accent).SingleLine().Shrink(0)
		}
		counts(c, f)
	})
	if b.Clicked() {
		v.openFile(f.Path)
	}
}

// fileBar heads the file open: back to the list, its path, its counts.
func (v *View) fileBar(c *ui.Context) {
	fi := v.fileIndex(v.file)
	if fi < 0 {
		return
	}
	f := v.diff.Files[fi]
	ui.Row(c).Height(34).Shrink(0).FillWidth().PaddingX(8).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		if widget.IconButton(c, icon.ArrowLeft, "All changes", false).Clicked() {
			v.openFile("")
		}
		letter, col := statusLetter(f.Status)
		ui.Text(c, letter).Font(theme.MonoFont).FontSize(12).TextColor(col).SingleLine().Shrink(0)
		name := f.Path
		if f.OldPath != "" {
			name = f.OldPath + " → " + f.Path
		}
		ui.Text(c, name).Font(theme.MonoFont).FontSize(12).SingleLine().Shrink(1)
		ui.Spacer(c)
		counts(c, f)
	})
}

// counts are a file's added and removed lines, as +n −n.
func counts(c *ui.Context, f proto.DiffFile) {
	if f.Added > 0 {
		ui.Text(c, "+"+strconv.Itoa(f.Added)).Font(theme.MonoFont).FontSize(11.5).TextColor(theme.Done).SingleLine().Shrink(0)
	}
	if f.Removed > 0 {
		ui.Text(c, "−"+strconv.Itoa(f.Removed)).Font(theme.MonoFont).FontSize(11.5).TextColor(theme.Error).SingleLine().Shrink(0)
	}
}

// statusLetter is how git's short status writes a file's change.
func statusLetter(status string) (string, ui.Color) {
	switch status {
	case proto.FileAdded:
		return "A", theme.Done
	case proto.FileDeleted:
		return "D", theme.Error
	case proto.FileRenamed:
		return "R", theme.Accent
	}
	return "M", theme.Warning
}

// diffLine is a line with its old and new numbers, tinted by what it is.
// Clicking it opens a review comment on it.
func (v *View) diffLine(c *ui.Context, file string, l proto.DiffLine) {
	bg, fg, mark := theme.Fade(theme.Background, 0), theme.TextSoft, " "
	switch l.Kind {
	case '+':
		bg, fg, mark = theme.Fade(theme.Done, 0.12), theme.Text, "+"
	case '-':
		bg, fg, mark = theme.Fade(theme.Error, 0.12), theme.Text, "−"
	}
	num := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	b := ui.ButtonBase(c).FillWidth().Height(rowHeight).Background(bg).Clip().Justify(ui.Start).Gap(0).
		Cursor(ui.CursorPointer).Label("Comment on line " + num(max(l.New, l.Old)))
	if b.Hovered() {
		b.Background(theme.Fade(theme.Accent, 0.10))
	}
	if b.Clicked() {
		v.startDraft(file, l)
	}
	b.Children(func() {
		for _, n := range []int{l.Old, l.New} {
			ui.Text(c, num(n)).Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).Width(40).PaddingX(5).
				TextAlign(ui.End).SingleLine().Shrink(0)
		}
		ui.Text(c, mark).Font(theme.MonoFont).FontSize(12).TextColor(theme.Muted).Width(14).SingleLine().Shrink(0)
		ui.Text(c, l.Text).Font(theme.MonoFont).FontSize(12).TextColor(fg).SingleLine().Shrink(1)
	})
}
