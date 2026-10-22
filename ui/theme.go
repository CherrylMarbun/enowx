package ui

import (
	"embed"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// Graphite palette. Status colors differ in lightness as well as hue.
var (
	colBackground = ui.Hex("#0E0F12") // window, terminals
	colPanel      = ui.Hex("#121418") // sidebar, title and status bars
	colRaised     = ui.Hex("#181B21") // cards, inputs
	colHover      = ui.Hex("#1A1E26")
	colSelected   = ui.Hex("#1E2330")
	colBorder     = ui.Hex("#1F2228")
	colBorderHi   = ui.Hex("#2A2E36")
	colText       = ui.Hex("#E6E7EA")
	colTextStrong = ui.Hex("#FFFFFF")
	colTextSoft   = ui.Hex("#B8BCC6")
	colMuted      = ui.Hex("#8B909A")
	colAccent     = ui.Hex("#7FA6FF")
	colOnAccent   = ui.Hex("#0E0F12")

	colWorking = ui.Hex("#7FA6FF")
	colDone    = ui.Hex("#5FD39A")
	colError   = ui.Hex("#FF7A80")
	colIdle    = ui.Hex("#6B7080")
)

const monoFont = "monospace"

// graphiteTheme is the dark theme every window uses, whatever the system's
// appearance: terminals and agents look best on a dark ground.
func graphiteTheme() *ui.Theme {
	t := *ui.DarkTheme()
	t.Background = colBackground
	t.Surface = colRaised
	t.SurfaceHover = colHover
	t.SurfacePressed = colSelected
	t.Border = colBorderHi
	t.Text = colText
	t.TextMuted = colMuted
	t.Accent = colAccent
	t.AccentText = colOnAccent
	t.Radius = 8
	t.Inverse, t.InverseText = ui.Hex("#2A2E36"), colText
	return &t
}

// terminalTheme is the terminals' colors: the app's ground and text with a
// palette tuned to it.
var terminalTheme = func() *terminal.Theme {
	t := terminal.DarkTheme()
	t.Foreground, t.Background = ui.Hex("#D5D8DE"), colBackground
	t.Cursor = ui.Hex("#D5D8DE")
	t.Selection = ui.Hex("#2B3448")
	t.Palette = [16]ui.Color{
		ui.Hex("#2A2E36"), ui.Hex("#FF7A80"), ui.Hex("#5FD39A"), ui.Hex("#F2C15B"),
		ui.Hex("#7FA6FF"), ui.Hex("#C792EA"), ui.Hex("#5FC8D3"), ui.Hex("#D5D8DE"),
		ui.Hex("#6B7080"), ui.Hex("#FF9A9F"), ui.Hex("#86E2B4"), ui.Hex("#F6D284"),
		ui.Hex("#A4C1FF"), ui.Hex("#DAB3F0"), ui.Hex("#8BDCE4"), ui.Hex("#FFFFFF"),
	}
	return t
}()

//go:embed icons/*.svg
var iconFiles embed.FS

func icon(name string) *ui.SVG {
	data, err := iconFiles.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return ui.MustParseSVG(data)
}

var (
	iconPlus         = icon("plus")
	iconChevronDown  = icon("chevron-down")
	iconChevronRight = icon("chevron-right")
	iconChevronsUD   = icon("chevrons-up-down")
	iconGit          = icon("git")
	iconFolder       = icon("folder")
	iconFolderPlus   = icon("folder-plus")
	iconServer       = icon("server")
	iconLaptop       = icon("laptop")
	iconMaximize     = icon("maximize")
	iconRestore      = icon("restore")
	iconTerminal     = icon("terminal")
	iconCheck        = icon("check")
	iconClose        = icon("x")
	iconTailscale    = icon("tailscale")
	iconTrash        = icon("trash")
	iconSync         = icon("sync")
	iconLock         = icon("lock")
	iconArrowUp      = icon("arrow-up")
	iconHome         = icon("home")
	agentIcons       = map[string]*ui.SVG{
		"claude":   icon("claude"),
		"codex":    icon("codex"),
		"opencode": icon("opencode"),
		"gemini":   icon("gemini"),
		"kiro":     icon("kiro"),
		"shell":    iconTerminal,
		"custom":   iconPlus,
	}
)

// agentIcon returns the icon of an agent ID, the terminal for unknown ones.
func agentIcon(id string) *ui.SVG {
	if s, ok := agentIcons[id]; ok {
		return s
	}
	return iconTerminal
}
