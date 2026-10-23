// Package icon holds the app's SVG icons, drawn in one stroke style.
package icon

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

//go:embed svg/*.svg
var files embed.FS

func load(name string) *ui.SVG {
	data, err := files.ReadFile("svg/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return ui.MustParseSVG(data)
}

// The icons.
var (
	Plus         = load("plus")
	PanelRight   = load("panel-right")
	ArrowLeft    = load("arrow-left")
	ArrowRight   = load("arrow-right")
	Browser      = load("browser")
	AppWindow    = load("app-window")
	Play         = load("play")
	Camera       = load("camera")
	Code         = load("code")
	External     = load("external")
	ChevronDown  = load("chevron-down")
	ChevronRight = load("chevron-right")
	ChevronsUD   = load("chevrons-up-down")
	Git          = load("git")
	Folder       = load("folder")
	FolderPlus   = load("folder-plus")
	Server       = load("server")
	Laptop       = load("laptop")
	Maximize     = load("maximize")
	Restore      = load("restore")
	Terminal     = load("terminal")
	Check        = load("check")
	Close        = load("x")
	Tailscale    = load("tailscale")
	Trash        = load("trash")
	Sync         = load("sync")
	Lock         = load("lock")
	ArrowUp      = load("arrow-up")
	Home         = load("home")
	Chat         = load("chat")
	Usage        = load("usage")
	Split        = load("split")
	More         = load("more")
	Search       = load("search")
	Alert        = load("alert")
	Palette      = load("palette")
	Sidebar      = load("sidebar")
	Credits      = load("credits")
	// Logo is enowX's mascot, traced to one shape so it takes any color.
	Logo     = load("logo")
	Collapse = load("collapse")
	// The chat's: tools, thoughts, the composer.
	Copy      = load("copy")
	Brain     = load("brain")
	File      = load("file")
	Pencil    = load("pencil")
	Globe     = load("globe")
	Stop      = load("stop")
	Paperclip = load("paperclip")
	Slash     = load("slash")
	CPU       = load("cpu")
	Shield    = load("shield")
	Gauge     = load("gauge")
	Bot       = load("agentbot")
	List      = load("list")
	agents    = map[string]*ui.SVG{
		"claude":   load("claude"),
		"codex":    load("codex"),
		"opencode": load("opencode"),
		"gemini":   load("gemini"),
		"kiro":     load("kiro"),
		"shell":    Terminal,
		"custom":   Plus,
	}
)

// Agent returns the icon of an agent ID, the terminal for unknown ones.
func Agent(id string) *ui.SVG {
	if s, ok := agents[id]; ok {
		return s
	}
	return Terminal
}
