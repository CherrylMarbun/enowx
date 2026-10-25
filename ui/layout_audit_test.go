package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/notesview"
)

// The layout audit draws the whole app, each page at a small, a common
// and a large window, and fails on layouts that break: text squeezed to a
// letter a line, text out of the window, a page's content huddled in a
// corner of a wide window. With SHOTS set it keeps a screenshot of each.

var auditSizes = [][2]int{{860, 620}, {1280, 800}, {1920, 1080}}

// auditPage is a page to draw, set up by open.
type auditPage struct {
	name string
	open func(a *App)
	// fill: the page's content should use the window's width (not a
	// column kept at a readable width, centered).
	fill bool
}

func auditPages() []auditPage {
	settings := func(section string) auditPage {
		return auditPage{name: "settings-" + strings.ToLower(strings.ReplaceAll(section, " ", "-")), open: func(a *App) {
			a.page, a.settingsSection = pageSettings, section
		}}
	}
	pages := []auditPage{
		{name: "sessions", open: func(a *App) { a.page = "" }},
		{name: "notes", open: func(a *App) { a.page = pageNotes }, fill: true},
		{name: "notes-page", open: func(a *App) {
			a.page = pageNotes
			a.notes.SetAssistant(false)
			a.notes.Select(a.notesOptions(), a.noteList.list[0].ID)
		}},
		{name: "notes-page-agent", open: func(a *App) {
			a.page = pageNotes
			a.notes.Select(a.notesOptions(), a.noteList.list[0].ID)
			a.notes.SetAssistant(true)
		}},
		{name: "hosts", open: func(a *App) { a.page = pageHosts }},
		{name: "usage", open: func(a *App) { a.page = pageUsage }, fill: true},
		{name: "vault", open: func(a *App) { a.page = pageVault }},
		{name: "canvas", open: func(a *App) { a.page = pageCanvas }, fill: true},
		{name: "appearance", open: func(a *App) { a.page = pageAppearance }},
		{name: "credits", open: func(a *App) { a.page = pageCredits }},
	}
	for _, s := range settingsSections {
		pages = append(pages, settings(s))
	}
	project := func(a *App) proto.ProjectInfo { return a.projects[0] }
	note := func(a *App) proto.NoteInfo { return a.noteList.list[0] }
	dialogs := []auditPage{
		{name: "dialog-new-session", open: func(a *App) { a.page = ""; a.openNewSession(project(a)) }},
		{name: "dialog-clone", open: func(a *App) { a.page = ""; a.openClone() }},
		{name: "dialog-add-host", open: func(a *App) { a.page = pageHosts; a.openAddHost() }},
		{name: "dialog-new-vps", open: func(a *App) { a.page = pageHosts; a.openCreateServer() }},
		{name: "dialog-import-notes", open: func(a *App) { a.page = pageNotes; a.openNoteImport(nil) }},
		{name: "dialog-export-notes", open: func(a *App) {
			a.page = pageNotes
			a.openNoteExport(notesview.ExportPick{All: true, Title: "All notes"})
		}},
		{name: "dialog-attach", open: func(a *App) { a.page = pageNotes; a.openAttach(note(a)) }},
		{name: "dialog-links", open: func(a *App) { a.page = ""; a.openLinks(project(a)) }},
		{name: "dialog-copy", open: func(a *App) { a.page = ""; a.openCopy(project(a)) }},
		{name: "dialog-project-sync", open: func(a *App) { a.page = ""; a.openProjectSync(project(a)) }},
		{name: "dialog-feature-ask", open: func(a *App) {
			a.page = pageSettings
			a.featAsks = []featureAsk{{req: proto.FeatureRequest{ID: "traffic", Reason: "to mock the payment API while testing the checkout"}, on: "this Mac"}}
		}},
	}
	return append(pages, dialogs...)
}

// closeAudit closes what the pages opened, for the next.
func closeAudit(a *App) {
	a.newSession.open = false
	a.notes.SetAssistant(false)
	a.clone, a.featAsks = nil, nil
	a.addHost.open = false
	a.noteImport, a.noteExport = nil, nil
	a.linksForm, a.attaching, a.copying, a.projectSyncing, a.creating = nil, nil, nil, nil, nil
}

func TestLayoutAudit(t *testing.T) {
	h := newNotesHarness(t)
	seedAudit(t, h)
	shots := os.Getenv("SHOTS")
	var problems []string
	for _, p := range auditPages() {
		for _, size := range auditSizes {
			w, hgt := size[0], size[1]
			closeAudit(h.app)
			p.open(h.app)
			tt := ui.NewTester(func(c *ui.Context) { h.app.view(c) }, w, hgt)
			h.tt = tt
			for range 6 {
				h.settle("", func() bool { return true })
			}
			name := fmt.Sprintf("%s-%dx%d", p.name, w, hgt)
			if shots != "" {
				shot(t, tt, filepath.Join("audit", name+".png"))
			}
			for _, pr := range layoutProblems(tt, float32(w), float32(hgt), p.fill) {
				problems = append(problems, name+": "+pr)
			}
		}
	}
	if len(problems) > 0 {
		t.Errorf("%d layout problems:\n%s", len(problems), strings.Join(problems, "\n"))
	}
}

// layoutProblems are what is wrong with the frame tt drew in a w×h
// window.
func layoutProblems(tt *ui.Tester, w, h float32, fill bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, text := range tt.Texts() {
		if seen[text] || strings.TrimSpace(text) == "" {
			continue
		}
		seen[text] = true
		r, ok := tt.Find(text)
		if !ok || r.W <= 0 || r.H <= 0 || r.W == r.H && r.W <= 44 {
			continue // an icon button's label, not a text
		}
		short := text
		if len(short) > 40 {
			short = short[:40] + "…"
		}
		words := strings.Fields(text)
		longest := 0
		for _, wd := range words {
			longest = max(longest, len([]rune(wd)))
		}
		// A word broken over lines: its box is narrower than the word
		// could be at the smallest text, about 5 points a letter.
		// Or a text squeezed to a word or two a line.
		lines := r.H / 15
		perLine := float32(len([]rune(text))) / max(lines, 1)
		if longest >= 5 && r.W < float32(longest)*4.5 && r.H > 26 || lines >= 3 && perLine < 9 && len(words) > 1 && r.W < 120 {
			out = append(out, fmt.Sprintf("%q squeezed to %.0f wide, %.0f tall", short, r.W, r.H))
		}
		if r.X+r.W > w+2 && r.X < w {
			out = append(out, fmt.Sprintf("%q runs out of the window (%.0f > %.0f)", short, r.X+r.W, w))
		}
	}
	return out
}

// seedAudit gives the app things to show: notes, a project, sessions.
func seedAudit(t *testing.T, h *notesHarness) {
	t.Helper()
	for _, n := range []string{"Release plan", "Groceries", "A note with a rather long title to see it wrap"} {
		h.cl.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: n}, Body: "# Heading\n\n- [ ] one\n- [x] two\n\nSome text."})
	}
	dir := t.TempDir()
	if _, err := h.cl.AddProject(dir); err != nil {
		t.Fatal(err)
	}
	h.app.setProjects([]proto.ProjectInfo{{Path: dir, Name: "antares"}})
	h.settle("the notes", func() bool { return len(h.app.noteList.list) >= 3 })
}
