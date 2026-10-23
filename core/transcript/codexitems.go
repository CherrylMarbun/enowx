package transcript

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// codexThreadItemRec is one item of a Codex conversation as Codex shows
// it, with the fields of every type it has.
type codexThreadItemRec struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	SummaryText []string `json:"summary_text"`
	Text        string   `json:"text"`

	// CommandExecution
	Command    []string `json:"command"`
	Status     string   `json:"status"`
	Output     string   `json:"aggregated_output"`
	ExitCode   *int     `json:"exit_code"`
	ParsedCmds []struct {
		Cmd string `json:"cmd"`
	} `json:"parsed_cmd"`

	// FileChange
	Changes map[string]struct {
		Type string `json:"type"`
		Diff string `json:"unified_diff"`
	} `json:"changes"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`

	// McpToolCall, DynamicToolCall, WebSearch, ImageView
	Server    string          `json:"server"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Result    *struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error json.RawMessage `json:"error"`
	Query string          `json:"query"`
	Path  string          `json:"path"`
}

// codexThreadItem adds a started item, or fills in the one it finishes,
// by the item's ID; a finished one whose start wasn't recorded is added.
func codexThreadItem(r *Reader, raw json.RawMessage, at time.Time) {
	var it codexThreadItemRec
	if json.Unmarshal(raw, &it) != nil {
		return
	}
	e, ok := codexItemEntry(it, at)
	if !ok {
		return
	}
	key := "item:" + it.ID
	if i, seen := r.tools[key]; seen {
		r.change(i, func(old *proto.TranscriptEntry) {
			e.Index, e.Time = old.Index, old.Time
			*old = e
		})
		return
	}
	r.tools[key] = r.add(e)
}

func codexItemEntry(it codexThreadItemRec, at time.Time) (proto.TranscriptEntry, bool) {
	e := proto.TranscriptEntry{Time: at}
	switch it.Type {
	case "UserMessage", "AgentMessage":
		var parts []string
		for _, c := range it.Content {
			if strings.EqualFold(c.Type, "text") && strings.TrimSpace(c.Text) != "" {
				parts = append(parts, strings.TrimSpace(c.Text))
			}
		}
		if len(parts) == 0 {
			return e, false
		}
		e.Kind, e.Text = proto.TranscriptAssistant, strings.Join(parts, "\n\n")
		if it.Type == "UserMessage" {
			e.Kind = proto.TranscriptUser
		}
	case "Reasoning":
		text := strings.TrimSpace(strings.Join(it.SummaryText, "\n\n"))
		if text == "" {
			return e, false
		}
		e.Kind, e.Text = proto.TranscriptThinking, text
	case "Plan":
		if strings.TrimSpace(it.Text) == "" {
			return e, false
		}
		e.Kind, e.Text, e.Detail = proto.TranscriptTool, "TodoWrite", clip(strings.TrimSpace(it.Text), 300)
		e.Done = true
	case "CommandExecution":
		e.Kind, e.Text = proto.TranscriptTool, "Bash"
		e.Detail = clip(codexCommandLine(it), 300)
		e.Result = clip(strings.TrimSpace(it.Output), 4000)
		e.Done = it.Status != "" && it.Status != "in_progress"
		e.Failed = it.Status == "failed" || it.Status == "declined" || it.ExitCode != nil && *it.ExitCode != 0
	case "FileChange":
		e.Kind, e.Text = proto.TranscriptTool, "Edit"
		paths := make([]string, 0, len(it.Changes))
		for p := range it.Changes {
			paths = append(paths, p)
		}
		slices.Sort(paths)
		var patch []string
		for _, p := range paths {
			patch = append(patch, it.Changes[p].Diff)
		}
		codexPatch(&e, strings.Join(patch, "\n"))
		names := make([]string, len(paths))
		for i, p := range paths {
			names[i] = filepath.Base(p)
		}
		if len(paths) == 1 {
			names[0] = paths[0]
		}
		e.Detail = clip(strings.Join(names, ", "), 300)
		e.Result = clip(strings.TrimSpace(it.Stdout+"\n"+it.Stderr), 4000)
		e.Done = it.Status != "" && it.Status != "in_progress"
		e.Failed = it.Status == "failed" || it.Status == "declined"
	case "McpToolCall", "DynamicToolCall":
		e.Kind, e.Text = proto.TranscriptTool, it.Tool
		if it.Server != "" {
			e.Text = "mcp__" + it.Server + "__" + it.Tool
		}
		e.Detail = toolDetail(it.Arguments)
		if it.Result != nil {
			var parts []string
			for _, c := range it.Result.Content {
				parts = append(parts, c.Text)
			}
			e.Result = clip(strings.TrimSpace(strings.Join(parts, "\n")), 4000)
		}
		if len(it.Error) > 0 && string(it.Error) != "null" {
			e.Failed, e.Result = true, clip(codexError(it.Error), 4000)
		}
		e.Done = it.Status != "" && it.Status != "inProgress" && it.Status != "in_progress"
		e.Failed = e.Failed || it.Status == "failed"
	case "WebSearch":
		e.Kind, e.Text, e.Detail, e.Done = proto.TranscriptTool, "WebSearch", clip(it.Query, 300), true
	case "ImageView":
		e.Kind, e.Text, e.Detail, e.Done = proto.TranscriptTool, "Read", strings.TrimPrefix(it.Path, "file://"), true
	default:
		return e, false
	}
	return e, true
}

// codexCommandLine is the command as typed: the script of a shell's -lc.
func codexCommandLine(it codexThreadItemRec) string {
	if len(it.ParsedCmds) == 1 && it.ParsedCmds[0].Cmd != "" {
		return it.ParsedCmds[0].Cmd
	}
	words := it.Command
	if len(words) == 3 && (words[1] == "-lc" || words[1] == "-c") {
		return words[2]
	}
	return strings.Join(words, " ")
}

func codexError(raw json.RawMessage) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Message != "" {
		return e.Message
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}
