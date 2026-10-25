package trafficview

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// sheet is a form of the tool as a modal dialog over the window: a title
// and a close button, the form, its buttons. Escape or a click outside
// closes it, as Cancel does.
func sheet(c *ui.Context, open *bool, width float32, title, description string, body, buttons func()) {
	w, _ := c.Size()
	width = min(width, max(280, w-32))
	widget.Sheet(c, open, width, func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(15)
			if description != "" {
				ui.Text(c, description).FontSize(11.5).TextColor(theme.Muted)
			}
		})
		if widget.IconButton(c, icon.Close, "Close", false).Clicked() {
			*open = false
		}
	}, body, func() {
		ui.Spacer(c)
		if widget.GhostButton(c, "Cancel").Clicked() {
			*open = false
		}
		buttons()
	})
}

// codeArea is a multi-line field of code: headers, a body.
func codeArea(c *ui.Context, label string, value *string, height float32) {
	widget.Field(c, label, func() {
		ui.TextAreaBase(c, value).FillWidth().MinHeight(height).MaxHeight(260).Padding(8, 10).Font(theme.MonoFont).FontSize(11.5).
			Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border)
	})
}

// errorText shows a form's error, when it has one.
func errorText(c *ui.Context, err string) {
	if err != "" {
		ui.Text(c, err).FontSize(12).TextColor(theme.Error)
	}
}

// check is a checkbox on a row of its own, at its start.
func check(c *ui.Context, value *bool, label string) {
	ui.Row(c).FillWidth().Children(func() { widget.Checkbox(c, value, label) })
}
