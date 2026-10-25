package notesview

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/iconlib"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Code and tables as a page shows them, their text not selectable, so a
// click on them opens them as text.

func codeView(c *ui.Context, lang, code string) {
	ui.Column(c).FillWidth().Radius(8).Background(theme.Sunken).Border(1, theme.Border).Children(func() {
		if lang != "" {
			ui.Text(c, lang).Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).Padding(8, 12, 0, 12)
		}
		text := code
		if text == "" {
			text = " "
		}
		ui.Text(c, text).Font(theme.MonoFont).FontSize(12.5).TextColor(theme.Text).Padding(10, 12).FillWidth()
	})
}

// drawings are the SVG drawings of pages shown, cleaned and parsed once,
// by their code; nil for code that doesn't draw. Main thread only.
var drawings = map[string]*ui.SVG{}

// drawingView is a code block of language svg as the drawing it is,
// cleaned first (core/iconlib): what runs or loads in it goes.
func drawingView(c *ui.Context, code string) {
	svg, ok := drawings[code]
	if !ok {
		if clean, err := iconlib.Sanitize([]byte(code), false); err == nil {
			svg, _ = ui.ParseSVG([]byte(clean))
		}
		if len(drawings) > 64 {
			clear(drawings)
		}
		drawings[code] = svg
	}
	if svg == nil {
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			codeView(c, "svg", code)
			ui.Text(c, "This SVG doesn't draw: fix its code.").FontSize(11.5).TextColor(theme.Error)
		})
		return
	}
	w, h := svg.Size()
	if w <= 0 || h <= 0 {
		w, h = 24, 24
	}
	// As large as it says, at most the page's width.
	const most = 640
	if w > most {
		w, h = most, h*most/w
	}
	ui.Row(c).FillWidth().Padding(6, 0).Children(func() {
		ui.Image(c, svg).Size(w, h).TextColor(theme.Text)
	})
}

func tableView(c *ui.Context, src string) {
	var rows [][]string
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.Trim(line, "|-: ") == "" {
			continue // the rule under the header
		}
		var cells []string
		for _, cell := range strings.Split(strings.Trim(line, "|"), "|") {
			cells = append(cells, strings.TrimSpace(cell))
		}
		rows = append(rows, cells)
	}
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	ui.Column(c).FillWidth().Radius(8).Border(1, theme.Border).Clip().Children(func() {
		for i, r := range rows {
			row := ui.Row(c.Key(i)).FillWidth()
			if i == 0 {
				row.Background(theme.Raised)
			}
			if i > 0 {
				row.BorderWidth(1, 0, 0, 0).BorderColor(theme.Border)
			}
			row.Children(func() {
				for j := range cols {
					cell := ""
					if j < len(r) {
						cell = r[j]
					}
					t := widget.InlineLabel(c.Key(j), cell).FontSize(13).Grow(1).Basis(0).Padding(7, 10)
					if i == 0 {
						t.FontWeight(600)
					}
				}
			})
		}
	})
}
