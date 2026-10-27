package widget

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
)

// CopyFeedback belongs to one copy action on the UI thread.
type CopyFeedback struct {
	until time.Time
}

// CopyIcon copies value and briefly confirms it with a check and tooltip.
func CopyIcon(c *ui.Context, state *CopyFeedback, svg *ui.SVG, label, tip, value string) {
	if remaining := state.until.Sub(c.Now()); remaining > 0 {
		svg = icon.Check
		label = strings.Replace(label, "Copy", "Copied", 1)
		tip = "Copied to clipboard"
		c.After(remaining)
	}
	b := IconButton(c, svg, label, false).Tooltip(tip)
	if c.Now().Before(state.until) {
		b.TextColor(theme.Done)
	}
	if b.Clicked() {
		c.WriteClipboard(value)
		state.until = c.Now().Add(1500 * time.Millisecond)
		c.After(0)
	}
}
