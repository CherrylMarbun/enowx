// Package canvasview is the Canvas page: the connected host's canvases
// (mockups, wireframes, diagrams, boards), each shown as its frames, which
// the user edits, comments on and exports, and watches agents design live.
// The app gives it the canvases and how to change them through Options.
package canvasview

import (
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Options are what the page shows and does: the host's canvases, and
// calls to it, whose done funcs run on the main thread.
type Options struct {
	Host     string
	Canvases []proto.CanvasInfo
	Loaded   bool
	Err      string
	Load     func(id string, done func(proto.Canvas, error))
	Create   func(p proto.CanvasCreateParams, done func(proto.Canvas, error))
	Delete   func(id string, done func(error))
	Edit     func(e proto.CanvasEdit, done func(proto.Canvas, error))
	History  func(id string, done func(proto.CanvasHistory, error))
	Version  func(id string, n int, done func(proto.Canvas, error))
	Undo     func(id string, done func(proto.Canvas, error))
	Redo     func(id string, done func(proto.Canvas, error))
	Restore  func(id string, n int, done func(proto.Canvas, error))
	Comments func(id string, done func([]proto.CanvasComment, error))
	Comment  func(p proto.CanvasCommentParams, done func([]proto.CanvasComment, error))
	Control  func(id, action string, done func(error))
	// Render renders a frame (or, for html without one, the canvas) as
	// format: png, pdf, svg or html.
	Render func(id, frame, format string, done func([]byte, error))
	// SaveFile asks where to save data, named name, and saves it.
	SaveFile func(name string, data []byte)
	// Projects are the host's projects, to attach a canvas to; MakeCode
	// has an agent turn canvas c into code in project.
	Projects []proto.ProjectInfo
	MakeCode func(c proto.Canvas, project string)
	// AgentName names the agent of a session.
	AgentName func(session string) string
	// Web shows a mockup's HTML: nil where this device has no web view.
	Web *Web
}

// designingFor is how long after an agent's change it counts as designing.
const designingFor = 8 * time.Second

// Modes of the stage.
const (
	modeDesign   = 0 // select, move, resize, edit
	modeInteract = 1 // the mockup works as the real thing
)

// View is the Canvas page's state. It belongs to the main thread.
type View struct {
	selected string
	canvas   *proto.Canvas // the canvas open, as last read
	loading  bool
	err      string
	frame    string // the frame shown: its ID
	mode     int
	// preview shows a mockup frame at a device's width, 0 its own.
	preview int
	scale   float32 // the stage's, fitting the frame: from the last frame
	sel     string  // the element selected
	// last is the latest change an agent made, highlighted a moment.
	last     proto.CanvasChange
	lastAt   time.Time
	panel    string // the right panel: "layers", "comments", "versions"
	comments []proto.CanvasComment
	history  *proto.CanvasHistory
	compare  *compareState
	drag     *dragState
	editing  *textEdit
	connect  string // a shape to draw an arrow from, picked first
	draft    string // a comment being written
	creating *createForm
	deleting *proto.CanvasInfo
	renaming *string
	menu     widget.ContextMenu
	filter   string
	htmlEdit *htmlEdit
	web      *Web
}

// WebOf is the view's web view of mockups, made by make the first time;
// nil where make gives none (no web view on this device).
func (v *View) WebOf(make func() *Web) *Web {
	if v.web == nil && make != nil {
		v.web = make()
	}
	return v.web
}

// Web is the view's web view of mockups, nil before WebOf made one.
func (v *View) Web() *Web { return v.web }

// Canvas is the canvas open as last read, nil for none.
func (v *View) Canvas() *proto.Canvas { return v.canvas }

// Selected is the canvas open, "" for none.
func (v *View) Selected() string { return v.selected }

// Open opens canvas id, as asked from elsewhere (an agent's canvas open).
func (v *View) Open(o Options, id string) {
	if v.selected == id && v.canvas != nil {
		return
	}
	v.selected, v.canvas, v.sel, v.frame, v.history, v.compare = id, nil, "", "", nil, nil
	v.loading = true
	o.Load(id, func(c proto.Canvas, err error) {
		v.loading = false
		if v.selected != id {
			return
		}
		if err != nil {
			v.err = err.Error()
			return
		}
		v.show(o, c)
		v.loadComments(o)
	})
}

// show takes c as the canvas open.
func (v *View) show(o Options, c proto.Canvas) {
	old := v.canvas
	v.canvas = &c
	if !slices.ContainsFunc(c.Frames, func(f proto.CanvasFrame) bool { return f.ID == v.frame }) && len(c.Frames) > 0 {
		v.frame = c.Frames[0].ID
	}
	if o.Web != nil {
		o.Web.show(c, v.frameOf(), old)
	}
}

// Apply takes a change the host told: the canvas open as it is now, and
// the element an agent changed, highlighted.
func (v *View) Apply(o Options, ch proto.CanvasChange) {
	if ch.Canvas.ID != v.selected {
		return
	}
	if ch.Session != "" {
		v.last, v.lastAt = ch, time.Now()
		if ch.Frame != "" && ch.Frame != v.frame && v.drag == nil && v.editing == nil {
			v.frame = ch.Frame // follow the agent to the frame it works on
		}
	}
	old := v.canvas
	c := ch.Canvas
	v.canvas = &c
	if !slices.ContainsFunc(c.Frames, func(f proto.CanvasFrame) bool { return f.ID == v.frame }) && len(c.Frames) > 0 {
		v.frame = c.Frames[0].ID
	}
	if o.Web != nil {
		o.Web.apply(c, v.frameOf(), old, ch)
	}
	if ch.Op == "comment" {
		v.loadComments(o)
	}
	if v.history != nil {
		v.loadHistory(o)
	}
}

func (v *View) loadComments(o Options) {
	id := v.selected
	o.Comments(id, func(list []proto.CanvasComment, err error) {
		if err == nil && v.selected == id {
			v.comments = list
		}
	})
}

func (v *View) loadHistory(o Options) {
	id := v.selected
	o.History(id, func(h proto.CanvasHistory, err error) {
		if err == nil && v.selected == id {
			v.history = &h
		}
	})
}

// frameOf is the frame shown.
func (v *View) frameOf() proto.CanvasFrame {
	if v.canvas == nil {
		return proto.CanvasFrame{}
	}
	for _, f := range v.canvas.Frames {
		if f.ID == v.frame {
			return f
		}
	}
	if len(v.canvas.Frames) > 0 {
		return v.canvas.Frames[0]
	}
	return proto.CanvasFrame{}
}

// designing says an agent is designing the canvas open: it changed it
// moments ago.
func (v *View) designing(now time.Time) bool {
	return v.canvas != nil && v.canvas.Agent != "" && now.Sub(v.canvas.AgentAt) < designingFor
}

// edit makes e to the canvas open, the canvas as the host has it after.
func (v *View) edit(o Options, e proto.CanvasEdit) {
	if v.canvas == nil {
		return
	}
	e.ID = v.canvas.ID
	if e.Frame == "" {
		e.Frame = v.frame
	}
	o.Edit(e, func(c proto.Canvas, err error) {
		if err != nil {
			v.err = err.Error()
			return
		}
		if v.canvas != nil && c.ID == v.canvas.ID && c.Version >= v.canvas.Version {
			v.show(o, c)
		}
	})
}

// Draw builds the page: the canvases beside the one open.
func (v *View) Draw(c *ui.Context, o Options) {
	if v.selected == "" && len(o.Canvases) > 0 && v.creating == nil {
		v.Open(o, o.Canvases[0].ID)
	}
	ui.Row(c).Grow(1).FillWidth().FillHeight().MinHeight(0).TextColor(theme.Text).Children(func() {
		v.sidebar(c, o)
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Background(theme.Background).Children(func() {
			switch {
			case v.canvas == nil && v.loading:
				ui.Text(c, "Opening…").FontSize(12).TextColor(theme.Muted).Padding(20)
			case v.canvas == nil:
				v.empty(c, o)
			default:
				v.page(c, o)
			}
		})
	})
	v.createDialog(c, o)
	v.deleteDialog(c, o)
	v.renameDialog(c, o)
}

// empty is the page without a canvas open.
func (v *View) empty(c *ui.Context, o Options) {
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		widget.EmptyState(c, icon.Canvas, "No canvas open",
			"Mockups, wireframes and diagrams, which you and agents design together: ask an agent for one, or make one here.",
			func() {
				if widget.Button(c, widget.Primary, nil, "New canvas", "").Clicked() {
					v.creating = &createForm{open: true, kind: 0}
				}
			})
		if o.Err != "" {
			ui.Text(c, o.Err).FontSize(12).TextColor(theme.Error)
		}
	})
}
