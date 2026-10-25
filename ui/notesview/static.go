package notesview

import (
	"strings"

	"github.com/egoist/mygo/ui"

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
