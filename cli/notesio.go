package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

const notesIOUsage = `usage: enowx-app notes import|import-status|import-cancel|export
  import FILE|DIR [--from notion|obsidian|md|enex|keep|html|enowx] [--group G] [--project P]
         [--mode skip|overwrite|duplicate] [--rows-as-notes] [--dry-run] [--no-wait]
                                   notes from other apps (a .zip or folder of a Notion export, an
                                   Obsidian vault, .md, .enex, Keep's Takeout, HTML, an enowX export);
                                   --dry-run lists what it would make; again, notes made before are
                                   skipped (--mode overwrite writes them again)
  import --from notion-api [--token-env VAR]
                                   every page a Notion integration sees; its token in $VAR
                                   (NOTION_TOKEN by default), never printed
  import-status [ID] [--wait]      how an import goes (none: the imports lately)
  import-cancel ID                 stop an import, the notes it made taken back
  export [NOTE] [--group G | --all] [--format md|zip|html|pdf|json] [--out PATH]
                                   notes as a file (here by default): md one note, zip folders of
                                   Markdown (Obsidian, Notion), json a full backup import restores`

// notesIO imports and exports notes.
func (r runner) notesIO(cmd string, args []string) error {
	if os.Getenv(proto.EnvNotesPropose) != "" {
		return errors.New("notes: here notes are only proposed: write or new, which the user accepts")
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	from := fs.String("from", "", "")
	group := fs.String("group", "", "")
	project := fs.String("project", "", "")
	mode := fs.String("mode", "", "")
	rows := fs.Bool("rows-as-notes", false, "")
	dry := fs.Bool("dry-run", false, "")
	noWait := fs.Bool("no-wait", false, "")
	wait := fs.Bool("wait", false, "")
	tokenEnv := fs.String("token-env", "NOTION_TOKEN", "")
	all := fs.Bool("all", false, "")
	format := fs.String("format", "", "")
	out := fs.String("out", "", "")
	arg, err := parseWithArg(fs, args)
	if err != nil {
		return errors.New(notesIOUsage)
	}
	switch cmd {
	case "import":
		p := proto.NoteImportParams{From: *from, Group: *group, Project: *project, Mode: *mode, DryRun: *dry, RowsAsNotes: *rows}
		if *from == proto.ImportNotionAPI {
			if p.Token = os.Getenv(*tokenEnv); p.Token == "" {
				return fmt.Errorf("import: $%s holds no Notion token (an integration's, from notion.so/my-integrations)", *tokenEnv)
			}
		} else {
			if arg == "" {
				return errors.New(notesIOUsage)
			}
			if p.Path, err = filepath.Abs(arg); err != nil {
				return err
			}
		}
		st, err := r.c.ImportNotes(p)
		if err != nil {
			return err
		}
		if *noWait {
			fmt.Fprintf(r.out, "Importing %s: \"enowx-app notes import-status %s --wait\" follows it.\n", st.Source, st.ID)
			return nil
		}
		return r.followImport(st.ID)
	case "import-status":
		if arg == "" {
			list, err := r.c.NoteImports()
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Fprintln(r.out, "No imports lately.")
			}
			for _, st := range list {
				fmt.Fprintf(r.out, "%s  %s  %s %d/%d\n", st.ID, st.Source, st.State, st.Done, st.Total)
			}
			return nil
		}
		if *wait {
			return r.followImport(arg)
		}
		st, err := r.c.NoteImport(arg)
		if err != nil {
			return err
		}
		return importResult(r.out, st)
	case "import-cancel":
		if arg == "" {
			return errors.New("usage: enowx-app notes import-cancel ID")
		}
		return r.c.CancelNoteImport(arg)
	case "export":
		p := proto.NoteExportParams{Group: *group, All: *all, Format: *format}
		if arg != "" {
			n, err := r.findNote(arg)
			if err != nil {
				return err
			}
			p.IDs = []string{n.ID}
		}
		if *out == "" {
			*out = "."
		}
		if p.Out, err = filepath.Abs(*out); err != nil {
			return err
		}
		ex, err := r.c.ExportNotes(p)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.out, "Exported %d notes to %s (%d bytes).\n", ex.Notes, ex.Path, ex.Size)
		return nil
	}
	return errors.New(notesIOUsage)
}

// followImport shows import id's progress until it ends.
func (r runner) followImport(id string) error {
	last := ""
	for {
		st, err := r.c.NoteImport(id)
		if err != nil {
			return err
		}
		if st.State != proto.ImportRunning {
			return importResult(r.out, st)
		}
		if line := fmt.Sprintf("%s %d/%d", st.Phase, st.Done, st.Total); line != last && st.Total > 0 {
			fmt.Fprintln(r.out, line)
			last = line
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// importResult says what an import made, would make, or why it stopped.
func importResult(w io.Writer, st proto.NoteImportStatus) error {
	switch st.State {
	case proto.ImportFailed:
		return errors.New(st.Error)
	case proto.ImportCanceled:
		return fmt.Errorf("the import of %s was cancelled; the notes it made were taken back", st.Source)
	case proto.ImportRunning:
		fmt.Fprintf(w, "Importing %s: %s %d/%d\n", st.Source, st.Phase, st.Done, st.Total)
		return nil
	}
	if st.DryRun && st.Preview != nil {
		pv := st.Preview
		fmt.Fprintf(w, "%s (%s) would make %d notes and %d files:\n", st.Source, pv.From, len(pv.Notes), pv.Assets)
		for _, e := range pv.Notes {
			where := ""
			if len(e.Path) > 0 {
				where = "  under " + strings.Join(e.Path, " › ")
			} else if e.Group != "" {
				where = "  in " + e.Group
			}
			fmt.Fprintf(w, "  %-9s %s%s\n", e.Action, e.Title, where)
		}
		if len(pv.Conflicts) > 0 {
			fmt.Fprintf(w, "Titles taken already (both kept): %s\n", strings.Join(pv.Conflicts, ", "))
		}
		return nil
	}
	rep := st.Report
	if rep == nil {
		fmt.Fprintf(w, "Imported %s.\n", st.Source)
		return nil
	}
	fmt.Fprintf(w, "Imported %d notes from %s", rep.Imported, st.Source)
	if rep.Overwritten > 0 {
		fmt.Fprintf(w, ", wrote %d again", rep.Overwritten)
	}
	fmt.Fprintln(w, ".")
	if len(rep.Notes) > 0 {
		fmt.Fprintf(w, "At the top: %s\n", strings.Join(rep.Notes, " "))
	}
	for _, s := range rep.Skipped {
		fmt.Fprintf(w, "Skipped %s: %s\n", s.Where, s.What)
	}
	for _, u := range rep.Unconverted {
		fmt.Fprintf(w, "Not converted in %s: %s\n", u.Where, u.What)
	}
	return nil
}

// notesIOTools are imports and exports as MCP tools.
var notesIOTools = []mcpTool{
	{name: "notes_import", description: "Import notes from other apps into the user's notes on this host: a Notion export (.zip or folder), an Obsidian vault or folder of Markdown, a .md/.txt, an Evernote .enex, Google Keep's Takeout, HTML (Apple Notes' exports), or an enowX export or backup. Waits until done and says what it made; dry_run lists what it would make first.",
		props: map[string]mcpProp{"path": {"string", "The file or folder, an absolute path on this host"},
			"from":    {"string", "notion, obsidian, md, enex, keep, html or enowx (found out when left out)"},
			"group":   {"string", "The group the notes go in (made if missing)"},
			"project": {"string", "The project they are for"}, "mode": {"string", "For notes imported before: skip (default), overwrite or duplicate"},
			"dry_run": {"boolean", "Only list what it would make"}, "rows_as_notes": {"boolean", "A Notion database as a note per row, not a table"}},
		required: []string{"path"},
		args: func(a map[string]any) []string {
			args := []string{"import", str(a, "path")}
			for _, k := range []string{"from", "group", "project", "mode"} {
				args = withFlag(args, a, k, k)
			}
			args = withFlag(args, a, "dry_run", "dry-run")
			return withFlag(args, a, "rows_as_notes", "rows-as-notes")
		}},
	{name: "notes_import_status", description: "How an import goes, or (no id) the imports lately.",
		props: map[string]mcpProp{"id": {"string", "The import's ID"}},
		args:  func(a map[string]any) []string { return []string{"import-status", str(a, "id")} }},
	{name: "notes_export", description: "Write notes as a file on this host: one note, a group's, or all; md (one note, or a .zip of several), zip (folders of Markdown with their files, which Obsidian opens and Notion imports), html, pdf, or json (a full backup notes_import restores). Answers the file's path.",
		props: map[string]mcpProp{"note": {"string", "A note's ID or title (with the notes under it)"}, "group": {"string", "A group's notes"},
			"all": {"boolean", "Every note"}, "format": {"string", "md, zip, html, pdf or json"}, "out": {"string", "The file or folder to write, an absolute path"}},
		required: []string{"out"},
		args: func(a map[string]any) []string {
			args := []string{"export"}
			if n := str(a, "note"); n != "" {
				args = append(args, n)
			}
			for _, k := range []string{"group", "all", "format", "out"} {
				args = withFlag(args, a, k, k)
			}
			return args
		}},
}
