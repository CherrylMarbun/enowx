package agent

import "strings"

// Choice is one value of a setting an agent can change while it runs: a
// model, a permission mode, an effort level.
type Choice struct {
	ID    string // the agent's own name for it ("opus", "acceptEdits")
	Label string
	Note  string
	// Shown is what the agent's screen says while this mode is on, for
	// modes switched by cycling (Settings.ModeKey).
	Shown string
	// Window is the context a model holds, in tokens, where it has been
	// checked; 0 when not known.
	Window int64
}

// Settings are what the chat's composer offers for an agent, and how each
// is changed in a running session.
type Settings struct {
	Models []Choice
	// ModelCommand, with %s the model's ID, switches the model.
	ModelCommand string
	Modes        []Choice
	// ModeKey cycles to the next mode, in the order of Modes; "" when the
	// mode can't be changed while the agent runs.
	ModeKey string
	Efforts []Choice
	// EffortCommand, with %s the level, sets the effort.
	EffortCommand string
}

// claudeSettings are Claude Code's, as of version 2.1.308 (claude --help;
// its /model menu, whose first ten entries these are; and its screen while
// Shift+Tab cycles: auto, manual, accept edits, plan, bypass when allowed,
// then auto again). The model IDs are the names its transcript records.
var claudeSettings = Settings{
	Models: []Choice{
		{ID: "default", Label: "Default", Note: "recommended: Opus 5.5 today"},
		// 1M: its status line shows 141k as 14%.
		{ID: "claude-opus-5-5", Label: "Opus 5.5", Note: "complex work and everyday tasks", Window: 1_000_000},
		{ID: "claude-fable-5-1", Label: "Fable 5.1", Note: "your toughest challenges"},
		{ID: "claude-sonnet-5-5", Label: "Sonnet 5.5", Note: "efficient for simpler tasks"},
		{ID: "claude-haiku-5-5", Label: "Haiku 5.5", Note: "fastest for quick answers"},
		{ID: "claude-haiku-4-5", Label: "Haiku 4.5", Note: "fast for quick answers"},
		{ID: "claude-sonnet-5", Label: "Sonnet 5", Note: "efficient for routine tasks"},
		{ID: "claude-opus-5", Label: "Opus 5", Note: "everyday, complex tasks"},
		{ID: "claude-fable-5", Label: "Fable 5", Note: "the hardest, longest-running tasks"},
		{ID: "claude-opus-4-8", Label: "Opus 4.8", Note: "everyday, complex tasks"},
	},
	ModelCommand: "/model %s",
	Modes: []Choice{
		{ID: "auto", Label: "Auto", Note: "a classifier approves safe actions", Shown: "auto mode on"},
		{ID: "default", Label: "Ask first", Note: "asks before edits and commands", Shown: "manual mode on"},
		{ID: "acceptEdits", Label: "Accept edits", Note: "edits files without asking", Shown: "accept edits on"},
		{ID: "plan", Label: "Plan", Note: "reads and plans, changes nothing", Shown: "plan mode on"},
		{ID: "bypassPermissions", Label: "Skip permissions", Note: "runs everything without asking", Shown: "bypass permissions on"},
	},
	ModeKey: "\x1b[Z", // Shift+Tab
	Efforts: []Choice{
		{ID: "low", Label: "Low"}, {ID: "medium", Label: "Medium"}, {ID: "high", Label: "High"},
		{ID: "xhigh", Label: "Extra high"}, {ID: "max", Label: "Max"},
	},
	EffortCommand: "/effort %s",
}

// longContext is the window of a model named for it ("[1m]").
const longContext = 1_000_000

// ContextWindow is how many tokens model holds, or 0 when it isn't known:
// agents don't record it, so it is known only for the models checked
// (Choice.Window) and those named for a long context.
func (s Settings) ContextWindow(model string) int64 {
	if strings.Contains(strings.ToLower(model), "[1m]") {
		return longContext
	}
	for _, m := range s.Models {
		if m.ID == model {
			return m.Window
		}
	}
	return 0
}

// SettingsOf returns what can be changed in a running session of the
// preset with the given ID; the zero Settings when nothing can.
func SettingsOf(id string) Settings {
	if id == "claude" {
		return claudeSettings
	}
	return Settings{}
}

// ModeShown is the mode the agent's screen says is on, or "" when it says
// none of them.
func (s Settings) ModeShown(screen string) string {
	// The indicator sits at the bottom: look from the end.
	lines := strings.Split(screen, "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-12; i-- {
		for _, m := range s.Modes {
			if m.Shown != "" && strings.Contains(lines[i], m.Shown) {
				return m.ID
			}
		}
	}
	return ""
}

// FindChoice returns the choice with the given ID, or one whose ID the given
// name contains, as a full model name contains its alias.
func FindChoice(list []Choice, name string) (Choice, bool) {
	for _, c := range list {
		if c.ID == name {
			return c, true
		}
	}
	for _, c := range list {
		if name != "" && strings.Contains(strings.ToLower(name), c.ID) {
			return c, true
		}
	}
	return Choice{}, false
}
