package ui

import (
	"path"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
)

// floatingSessionIdentity keeps a shell tab recognizable as its parent
// session. A tab running a different agent keeps its own identity.
func (a *App) floatingSessionIdentity(s proto.SessionInfo) proto.SessionInfo {
	svg := floatingSessionIcon(s)
	if s.Parent == "" || (svg != icon.Terminal && svg != icon.Plus) {
		return s
	}
	parent, ok := a.sessionByID(s.Parent)
	if !ok {
		for _, watch := range a.watches {
			for _, candidate := range watch.status.sessions {
				if candidate.ID == s.Parent {
					parent, ok = candidate, true
					break
				}
			}
			if ok {
				break
			}
		}
	}
	if ok && parent.Agent != "" && parent.Agent != "shell" && parent.Agent != "custom" {
		s.Agent = parent.Agent
		if s.Name == "" {
			s.Name = parent.Name
		}
	}
	return s
}

// floatingSessionIcon also recognizes agents started as custom commands.
// Explicit agent metadata wins; arbitrary shell scripts remain terminals.
func floatingSessionIcon(s proto.SessionInfo) *ui.SVG {
	if s.Agent != "" && s.Agent != "shell" && s.Agent != "custom" {
		return icon.Agent(s.Agent)
	}
	if len(s.Command) > 0 {
		// A remote host's path may use different separators and executable
		// suffixes from the device drawing this menu.
		command := strings.ToLower(path.Base(strings.ReplaceAll(s.Command[0], `\`, "/")))
		for _, suffix := range []string{".exe", ".cmd", ".bat", ".ps1"} {
			command = strings.TrimSuffix(command, suffix)
		}
		for _, preset := range agent.Presets {
			if command == strings.ToLower(preset.Command) {
				return icon.Agent(preset.ID)
			}
		}
	}
	return icon.Agent(s.Agent)
}

// floatingCaption keeps the action ring readable without opening a tooltip.
// Session names stay in the hub so long titles never crowd adjacent slices.
func floatingCaption(act floatingAction) string {
	switch act.key {
	case floatingActiveFan:
		return "Sessions"
	case "new":
		return "New session"
	case "open":
		return "Folder"
	case "palette":
		return "Search"
	default:
		return act.short
	}
}
