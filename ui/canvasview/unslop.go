package canvasview

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/unslopview"
)

// A redesign under way (Options.UnslopRun), over its design: the panel
// (unslopview.Panel), and in the page the reads, the regions and what
// each pass changes (unslop_web.go).

// unslopHUD draws o.UnslopRun's panel over the stage, when it is about
// design f.
func (v *View) unslopHUD(c *ui.Context, o Options, f proto.CanvasFrame) {
	run := o.UnslopRun
	if o.Web != nil {
		if o.Web.pass = ""; run != nil && run.Frame == f.ID && !run.Done {
			o.Web.pass = run.Pass
		}
		o.Web.scan(run, f.ID)
		o.Web.outlineRegions(run, f.ID, o.AgentName)
	}
	switch {
	case o.Unslopping:
		unslopview.DrawStarting(c, "Copying the design, reading it…")
		return
	case o.UnslopErr != "":
		unslopview.DrawError(c, o.UnslopErr, o.UnslopDismiss)
		return
	}
	if run == nil || run.Frame != f.ID {
		return
	}
	po := unslopview.PanelOptions{Reading: "Reading the design…", AgentName: o.AgentName,
		AgentState: o.AgentState, OpenSession: o.OpenSession, Viewing: o.Viewing, OpenReviewer: o.OpenReviewer}
	if run.Original != "" && o.UnslopCompare != nil {
		po.Compare = func() { o.UnslopCompare(run.ID, *run) }
	}
	v.hud.Draw(c, run, po)
}
