package fileview

import (
	"path"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Doc is a file of a session's folder, shown in a tab of its own: its
// lines, numbered. It belongs to the main thread; its poller reports
// through Options.Update.
type Doc struct {
	path    string // in the session's folder
	content *proto.FileContent
	lines   []string
	err     string
	polling bool
	list    ui.ListState
}

// NewDoc returns the doc of the file at p, not read yet.
func NewDoc(p string) *Doc { return &Doc{path: p} }

// Path is the file's path in the session's folder.
func (d *Doc) Path() string { return d.path }

// follow reads the file while the doc shows, unless it already does.
func (d *Doc) follow(o Options) {
	if d.polling || o.Client == nil {
		return
	}
	d.polling = true
	cl, id, p := o.Client, o.Session.ID, d.path
	go func() {
		for {
			f, err := cl.ReadFile(id, p)
			keep := make(chan bool, 1)
			o.Update(func() {
				if !o.Wanted() {
					d.polling = false
					keep <- false
					return
				}
				keep <- true
				if err != nil {
					d.err = client.Explain(err)
					return
				}
				if d.content == nil || f.Text != d.content.Text || f.Size != d.content.Size {
					d.content, d.lines = &f, splitLines(f.Text)
				}
				d.err = ""
			})
			if !<-keep {
				return
			}
			time.Sleep(refreshEvery)
		}
	}()
}

// Draw shows the file: a bar with its path and what to do with it, then
// its lines.
func (d *Doc) Draw(c *ui.Context, o Options) {
	d.follow(o)
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		d.bar(c, o)
		if d.err != "" {
			ui.Row(c).FillWidth().Padding(8, 14).Background(theme.Fade(theme.Error, 0.1)).Children(func() {
				ui.Text(c, d.err).FontSize(12).TextColor(theme.Error).Shrink(1)
			})
		}
		d.text(c)
	})
}

// bar is the file's path, its size, and copying it or opening it in the
// editor.
func (d *Doc) bar(c *ui.Context, o Options) {
	ui.Row(c).Height(42).Shrink(0).FillWidth().Padding(0, 10, 0, 14).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		dir, name := path.Split(d.path)
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			if dir != "" {
				ui.Text(c, dir).Font(theme.MonoFont).FontSize(12).TextColor(theme.Muted).SingleLine().Shrink(1)
			}
			ui.Text(c, name).Font(theme.MonoFont).FontSize(12).SingleLine().Shrink(0)
		})
		if d.content != nil {
			ui.Text(c, textfmt.Bytes(uint64(d.content.Size))).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(0)
		}
		abs := ""
		if o.Session.Dir != "" {
			abs = path.Join(o.Session.Dir, d.path) // the host's path, slashes and all
		}
		if abs != "" && widget.IconButton(c, icon.Copy, "Copy the path", false).Clicked() {
			c.WriteClipboard(abs)
		}
		if abs != "" && o.OpenInEditor != nil && widget.IconButton(c, icon.Pencil, "Open in "+o.EditorName, false).Clicked() {
			o.OpenInEditor(abs, 0)
		}
	})
}

// text is the file's lines, numbered, or why it shows none.
func (d *Doc) text(c *ui.Context) {
	f := d.content
	switch {
	case f == nil && d.err == "":
		centered(c, "Reading…")
		return
	case f == nil:
		return
	case f.Binary:
		centered(c, "A binary file: nothing to show as text.")
		return
	case len(d.lines) == 0:
		centered(c, "This file is empty.")
		return
	}
	lines := d.lines
	n := len(lines)
	if f.Truncated {
		n++
	}
	width := float32(len(strconv.Itoa(len(lines))))*7.5 + 16
	ui.List(c.Key("doc"), &d.list, n, func(i int) {
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
