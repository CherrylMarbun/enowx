package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/enowx/enowx/proto"
)

const notesUsage = `usage: enowx-app notes COMMAND
  (none)                           the host's notes, as a tree: ID, then title
  show NOTE                        a note's Markdown (NOTE: its ID or title)
  new TITLE [--parent NOTE] [--icon EMOJI] [--body TEXT|-]
                                   a new note, its ID printed; --body - reads stdin
  write NOTE [--title T] [--icon EMOJI] [--body TEXT|- | --append TEXT]
                                   change a note: its body replaced, or added to
  move NOTE [--parent NOTE|--top] [--before NOTE]
  rm NOTE                          delete a note and the notes under it`

// notes reads and writes the host's notes, the Notes page's.
func (r runner) notes(args []string) error {
	if len(args) == 0 {
		return r.notesTree()
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	parent := fs.String("parent", "", "")
	icon := fs.String("icon", "", "")
	body := fs.String("body", "", "")
	appendText := fs.String("append", "", "")
	title := fs.String("title", "", "")
	before := fs.String("before", "", "")
	top := fs.Bool("top", false, "")
	arg, err := parseWithArg(fs, rest)
	if err != nil {
		return errors.New(notesUsage)
	}
	if *body == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		*body = string(data)
	}
	propose := os.Getenv(proto.EnvNotesPropose) != ""
	switch cmd {
	case "rm", "move":
		if propose {
			return errors.New("notes: here notes are only proposed: write or new, which the user accepts")
		}
	}
	switch cmd {
	case "help":
		fmt.Fprintln(r.out, notesUsage)
		return nil
	case "show":
		n, err := r.findNote(arg)
		if err != nil {
			return err
		}
		full, err := r.c.Note(n.ID)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.out, "# %s%s\n\n%s", iconPrefix(full.Icon), full.Title, full.Body)
		if !strings.HasSuffix(full.Body, "\n") {
			fmt.Fprintln(r.out)
		}
		return nil
	case "new":
		if arg == "" {
			return errors.New(notesUsage)
		}
		n := proto.Note{NoteInfo: proto.NoteInfo{Title: arg, Icon: *icon}, Body: *body}
		if *parent != "" {
			p, err := r.findNote(*parent)
			if err != nil {
				return err
			}
			n.Parent = p.ID
		}
		if propose {
			return r.proposeNote(n)
		}
		info, err := r.c.SaveNote(n)
		if err != nil {
			return err
		}
		fmt.Fprintln(r.out, info.ID)
		return nil
	case "write":
		n, err := r.findNote(arg)
		if err != nil {
			return err
		}
		full, err := r.c.Note(n.ID)
		if err != nil {
			return err
		}
		set := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
		if set["title"] {
			full.Title = *title
		}
		if set["icon"] {
			full.Icon = *icon
		}
		switch {
		case set["body"]:
			full.Body = *body
		case set["append"]:
			if full.Body != "" && !strings.HasSuffix(full.Body, "\n") {
				full.Body += "\n"
			}
			full.Body += *appendText + "\n"
		}
		if propose {
			return r.proposeNote(full)
		}
		_, err = r.c.SaveNote(full)
		return err
	case "move":
		n, err := r.findNote(arg)
		if err != nil {
			return err
		}
		to := n.Parent
		switch {
		case *top:
			to = ""
		case *parent != "":
			p, err := r.findNote(*parent)
			if err != nil {
				return err
			}
			to = p.ID
		}
		b := ""
		if *before != "" {
			bn, err := r.findNote(*before)
			if err != nil {
				return err
			}
			b = bn.ID
		}
		return r.c.MoveNote(n.ID, to, b)
	case "rm":
		n, err := r.findNote(arg)
		if err != nil {
			return err
		}
		return r.c.DeleteNote(n.ID)
	}
	return errors.New(notesUsage)
}

// proposeNote proposes n for the user to accept in the Notes page.
func (r runner) proposeNote(n proto.Note) error {
	p, err := r.c.ProposeNote(n, os.Getenv("ENOWX_SESSION"))
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "Proposed (%s): the user sees it in Notes and accepts or rejects it.\n", p.ID)
	return nil
}

func iconPrefix(icon string) string {
	if icon == "" {
		return ""
	}
	return icon + " "
}

// notesTree prints the notes, each under its parent.
func (r runner) notesTree() error {
	list, err := r.c.Notes()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Fprintln(r.out, "No notes yet: \"enowx-app notes new TITLE\" makes one.")
		return nil
	}
	children := map[string][]proto.NoteInfo{}
	for _, n := range list {
		children[n.Parent] = append(children[n.Parent], n)
	}
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, n := range children[parent] {
			fmt.Fprintf(r.out, "%s  %s%s%s\n", n.ID, strings.Repeat("  ", depth), iconPrefix(n.Icon), n.Title)
			walk(n.ID, depth+1)
		}
	}
	walk("", 0)
	return nil
}

// findNote is the note whose ID or title (any case) is ref.
func (r runner) findNote(ref string) (proto.NoteInfo, error) {
	if ref == "" {
		return proto.NoteInfo{}, errors.New(notesUsage)
	}
	list, err := r.c.Notes()
	if err != nil {
		return proto.NoteInfo{}, err
	}
	var found []proto.NoteInfo
	for _, n := range list {
		if n.ID == ref {
			return n, nil
		}
		if strings.EqualFold(n.Title, ref) {
			found = append(found, n)
		}
	}
	switch len(found) {
	case 0:
		return proto.NoteInfo{}, fmt.Errorf("no note %q: \"enowx-app notes\" lists them", ref)
	case 1:
		return found[0], nil
	}
	return proto.NoteInfo{}, fmt.Errorf("%d notes are titled %q: use the ID (\"enowx-app notes\")", len(found), ref)
}

// noteTools are the notes as MCP tools.
var noteTools = []mcpTool{
	{name: "notes_list", description: "The user's notes on this host (the Notes page in enowX), as a tree: each note's ID and title.",
		args: func(map[string]any) []string { return nil }},
	{name: "notes_read", description: "A note's Markdown.",
		props: map[string]mcpProp{"note": {"string", "The note's ID or title"}}, required: []string{"note"},
		args: func(a map[string]any) []string { return []string{"show", str(a, "note")} }},
	{name: "notes_create", description: "A new note in the user's notes, in Markdown (task lists, headings, code…); answers its ID.",
		props: map[string]mcpProp{"title": {"string", "Its title"}, "body": {"string", "Its Markdown"}, "parent": {"string", "The note it goes under (ID or title)"},
			"icon": {"string", "An emoji"}},
		required: []string{"title"},
		args: func(a map[string]any) []string {
			args := []string{"new", str(a, "title")}
			for _, k := range []string{"body", "parent", "icon"} {
				args = withFlag(args, a, k, k)
			}
			return args
		}},
	{name: "notes_write", description: "Change a note: replace its body, add to its end, or rename it.",
		props: map[string]mcpProp{"note": {"string", "The note's ID or title"}, "body": {"string", "Its new Markdown, replacing all of it"},
			"append": {"string", "Markdown added at its end"}, "title": {"string", "A new title"}, "icon": {"string", "A new emoji"}},
		required: []string{"note"},
		args: func(a map[string]any) []string {
			args := []string{"write", str(a, "note")}
			for _, k := range []string{"body", "append", "title", "icon"} {
				args = withFlag(args, a, k, k)
			}
			return args
		}},
	{name: "notes_delete", description: "Delete a note and the notes under it.",
		props: map[string]mcpProp{"note": {"string", "The note's ID or title"}}, required: []string{"note"},
		args: func(a map[string]any) []string { return []string{"rm", str(a, "note")} }},
}
