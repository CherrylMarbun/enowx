package chatview

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// turn is what came of one message of the user's: the message, the work
// the agent did for it (thoughts, tool calls, what it said on the way),
// and its answer, the text it ended on.
type turn struct {
	start  int // index of the turn's first entry, which keys it
	prompt *proto.TranscriptEntry
	work   []proto.TranscriptEntry
	answer []proto.TranscriptEntry
}

// splitTurns groups entries into turns. Notices stand as turns of their
// own, without a prompt.
func splitTurns(entries []proto.TranscriptEntry) []turn {
	var out []turn
	for i := range entries {
		e := entries[i]
		if e.Kind == proto.TranscriptUser || len(out) == 0 || e.Kind == proto.TranscriptNotice {
			out = append(out, turn{start: e.Index})
		}
		t := &out[len(out)-1]
		switch e.Kind {
		case proto.TranscriptUser:
			t.prompt = &entries[i]
		case proto.TranscriptNotice:
			t.answer = append(t.answer, e)
		default:
			t.work = append(t.work, e)
		}
	}
	// The answer is the text after the last thought or tool call.
	for i := range out {
		t := &out[i]
		last := len(t.work)
		for last > 0 && t.work[last-1].Kind == proto.TranscriptAssistant {
			last--
		}
		t.answer = append(t.answer, t.work[last:]...)
		t.work = t.work[:last]
	}
	return out
}

// elapsed is how long the turn's work took, from its prompt to its last
// entry.
func (t turn) elapsed() time.Duration {
	if t.prompt == nil || t.prompt.Time.IsZero() {
		return 0
	}
	end := t.prompt.Time
	for _, e := range append(append([]proto.TranscriptEntry{}, t.work...), t.answer...) {
		if e.Time.After(end) {
			end = e.Time
		}
	}
	return end.Sub(t.prompt.Time)
}

// turn draws one turn; animate makes it come in, for turns that arrive
// while the chat shows, not those it opens on.
func (ch *Chat) turn(c *ui.Context, t turn, running, animate bool) {
	col := ui.Column(c).FillWidth().Gap(12)
	if animate {
		col.Transition(widget.RowMotion(0))
	}
	col.Children(func() {
		if t.prompt != nil {
			ch.userMessage(c, *t.prompt, running)
		}
		if len(t.work) > 0 || running {
			ch.work(c.Key("work"), t, running)
		}
		for i, e := range t.answer {
			if e.Kind == proto.TranscriptNotice {
				ui.Row(c.Key(i)).FillWidth().Justify(ui.Center).Children(func() {
					ui.Text(c, e.Text).FontSize(11).TextColor(theme.Muted)
				})
				continue
			}
			widget.MarkdownLinks(c.Key(i), e.Text, ch.links)
		}
	})
}

// userMessage is the user's message: a soft pill on the right, with a
// pencil beside it, while the pointer is over it, that puts it in the
// composer to send again changed.
func (ch *Chat) userMessage(c *ui.Context, e proto.TranscriptEntry, running bool) {
	editing := ch.editing != nil && ch.editing.Index == e.Index
	row := ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center)
	row.Children(func() {
		ui.Box(c).Grow(1).MinWidth(60)
		// The pencil's place is kept while it hides, so the message
		// doesn't move or wrap anew under the pointer.
		if ch.editable {
			ui.Row(c).Size(28, 28).Shrink(0).Center().Children(func() {
				if !row.Hovered() && !editing {
					return
				}
				tip := "Edit and send again"
				if running {
					tip = "Edit once the agent is done"
				}
				if widget.IconButton(c, icon.Pencil, tip, running).Clicked() && !running {
					ch.edit(e)
				}
			})
		}
		bubble := ui.Column(c).Shrink(1).MaxWidth(560).Padding(9, 14).Radius(16).Background(theme.Raised).Border(1, theme.Border)
		if editing {
			bubble.Border(1, theme.AccentLine)
		}
		text := e.Text
		if ch.showUser != nil {
			text = ch.showUser(text)
		}
		bubble.Children(func() {
			ui.Text(c, text).TextColor(theme.TextStrong).Selectable()
		})
	})
}

// edit puts message e in the composer, to send in its place.
func (ch *Chat) edit(e proto.TranscriptEntry) {
	ch.editing, ch.editErr = &e, ""
	ch.input, ch.focus = e.Text, true
}

// editBack is how many of the user's messages the one edited is from the
// end, itself counted: those the agent goes back over.
func (ch *Chat) editBack() int {
	n := 0
	for _, e := range ch.entries {
		if e.Kind == proto.TranscriptUser && e.Index >= ch.editing.Index {
			n++
		}
	}
	return n
}

// work is a turn's work folded into one line, "Worked for 12s", that
// opens to the thoughts and tool calls. The turn under way shows it open,
// headed by what it did so far: the chip over the composer says it is
// working, and for how long.
func (ch *Chat) work(c *ui.Context, t turn, running bool) {
	if running && len(t.work) == 0 {
		return
	}
	key := t.start
	open, set := ch.open[key]
	if !set {
		open = running
	}
	tools, edited := 0, map[string]bool{}
	for _, e := range t.work {
		if e.Kind == proto.TranscriptTool {
			tools++
			if e.Added+e.Removed > 0 {
				edited[e.Detail] = true
			}
		}
	}
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		head := ui.ButtonBase(c).Gap(7).Cursor(ui.CursorPointer).Label("Work").TextColor(theme.Muted)
		if head.Hovered() {
			head.TextColor(theme.TextSoft)
		}
		head.Children(func() {
			chev := icon.ChevronRight
			if open {
				chev = icon.ChevronDown
			}
			ui.Icon(c, chev).FontSize(11)
			var parts []string
			if tools > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", tools, plural(tools, "tool", "tools")))
			}
			if n := len(edited); n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s edited", n, plural(n, "file", "files")))
			}
			if running {
				if len(parts) == 0 {
					parts = []string{"Thinking"}
				}
				ui.Text(c, strings.Join(parts, " · ")).FontSize(12.5).SingleLine()
				return
			}
			label := "Worked"
			if d := t.elapsed().Round(time.Second); d > 0 {
				label += " for " + d.String()
			}
			ui.Text(c, label).FontSize(12.5).SingleLine()
			if len(parts) > 0 {
				ui.Text(c, "· "+strings.Join(parts, " · ")).FontSize(12).TextColor(theme.Idle).SingleLine()
			}
		})
		if head.Clicked() {
			ch.open[key] = !open
		}
		if !open || len(t.work) == 0 {
			return
		}
		steps := workSteps(t.work)
		widget.Card(c).Gap(0).Padding(4).Children(func() {
			for i, st := range steps {
				last := running && i == len(steps)-1
				switch {
				case st.entry != nil:
					ch.workItem(c.Key(st.entry.Index), *st.entry, last)
				case len(st.tools) >= groupFrom:
					ui.Column(c.Key(st.tools[0].Index)).FillWidth().Children(func() { ch.toolGroup(c, st.tools, last) })
				default:
					for j, e := range st.tools {
						ch.toolRow(c.Key(e.Index), e, last && j == len(st.tools)-1)
					}
				}
			}
		})
	})
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// workItem is one thought, tool call or remark inside a turn's work.
func (ch *Chat) workItem(c *ui.Context, e proto.TranscriptEntry, latest bool) {
	switch e.Kind {
	case proto.TranscriptThinking:
		label := "Thought"
		if e.DurationMs > 0 {
			label += " for " + (time.Duration(e.DurationMs) * time.Millisecond).Round(time.Second).String()
		}
		ch.row(c, e.Index, icon.Brain, label, "", nil, func() {
			ui.Text(c, e.Text).FontSize(12).TextColor(theme.Muted).Italic().Selectable()
		})
	case proto.TranscriptAssistant:
		ui.Column(c).FillWidth().Padding(6, 10).Children(func() {
			widget.InlineLinks(c, ch.links, e.Text).FontSize(12.5).TextColor(theme.TextSoft)
		})
	case proto.TranscriptTool:
		ch.toolRow(c, e, latest)
	}
}

// row is a line of a turn's work that opens to show body.
func (ch *Chat) row(c *ui.Context, index int, svg *ui.SVG, label, detail string, status func(), body func()) {
	open := ch.open[index]
	ui.Column(c).FillWidth().Children(func() {
		if ch.lineHead(c, svg, label, detail, status).Clicked() {
			ch.open[index] = !open
		}
		if open {
			ui.Column(c).FillWidth().Padding(2, 10, 8, 32).Transition(widget.PageMotion).Children(body)
		}
	})
}

// lineHead is the clickable line of a step: an icon, what it is, a detail
// in the code font, and its status at the end.
func (ch *Chat) lineHead(c *ui.Context, svg *ui.SVG, label, detail string, status func()) ui.Element {
	head := ui.ButtonBase(c).FillWidth().Gap(9).Padding(5, 10).Radius(theme.RadiusControl).Cursor(ui.CursorPointer).
		Justify(ui.Start).Label(label)
	bg := theme.Fade(theme.Hover, 0)
	if head.Hovered() {
		bg = theme.Hover
	}
	head.Background(widget.Tint(head, "bg", bg)).Children(func() {
		ui.Icon(c, svg).FontSize(13).TextColor(theme.Muted)
		ui.Text(c, label).FontSize(12.5).TextColor(theme.TextSoft).SingleLine().Shrink(0)
		if detail != "" {
			ui.Text(c, detail).Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(1).Grow(1)
		} else {
			ui.Spacer(c)
		}
		if status != nil {
			ui.Row(c).Gap(5).Shrink(0).Children(status)
		}
	})
	return head
}

// toolLook is the icon and verb a tool call shows with.
func toolLook(name string) (*ui.SVG, string) {
	switch name {
	case "Read", "NotebookRead":
		return icon.File, "Read"
	case "Edit", "MultiEdit", "NotebookEdit":
		return icon.Pencil, "Edit"
	case "Write":
		return icon.Pencil, "Write"
	case "Bash", "BashOutput", "KillShell":
		return icon.Terminal, "Run"
	case "Grep", "Glob", "LS":
		return icon.Search, "Search"
	case "WebFetch", "WebSearch":
		return icon.Globe, "Web"
	case "Task", "Agent":
		return icon.Bot, "Agent"
	case "TodoWrite":
		return icon.List, "Plan"
	}
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.Split(name, "__")
		return icon.Bot, parts[len(parts)-1]
	}
	return icon.Terminal, name
}

// toolBody is what an open tool call shows: the change an edit made, else
// what the tool answered.
func toolBody(c *ui.Context, e proto.TranscriptEntry) {
	if e.Old != "" || e.New != "" {
		var b strings.Builder
		for _, l := range strings.Split(e.Old, "\n") {
			if e.Old != "" {
				b.WriteString("-" + l + "\n")
			}
		}
		for _, l := range strings.Split(e.New, "\n") {
			b.WriteString("+" + l + "\n")
		}
		widget.CodeBlock(c, "diff", strings.TrimSuffix(b.String(), "\n"))
		return
	}
	if e.Result == "" {
		ui.Text(c, "No output yet.").FontSize(12).TextColor(theme.Muted)
		return
	}
	ui.Text(c, e.Result).Font(theme.MonoFont).FontSize(11).TextColor(theme.TextSoft).Padding(8, 10).Radius(theme.RadiusControl).
		Background(theme.Sunken).Border(1, theme.Border).FillWidth().Selectable()
}
