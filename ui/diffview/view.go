package diffview

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Draw shows the view: a bar of where the branch stands and what to do
// with it, then the diff.
func (v *View) Draw(c *ui.Context, o Options) {
	v.follow(o)
	if v.asked != "" && v.loaded && v.busy == "" {
		v.start(o, v.asked)
		v.asked = ""
	}
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		v.toolbar(c, o)
		v.conflictBanner(c, o)
		if v.err != "" {
			ui.Row(c).FillWidth().PaddingX(14).Padding(8, 14).Background(theme.Fade(theme.Error, 0.1)).Children(func() {
				ui.Text(c, v.err).FontSize(12).TextColor(theme.Error).Shrink(1)
			})
		}
		switch {
		case !v.loaded:
			centered(c, "Reading the changes…")
		case len(v.diff.Files) == 0 && v.err == "":
			msg := "Nothing changed since the last commit."
			if v.status.Base != "" {
				msg = "Nothing differs from " + v.status.Base + "."
			}
			centered(c, msg)
		default:
			if v.file != "" {
				v.fileBar(c)
			}
			v.lines(c)
		}
		v.commentBar(c, o)
	})
	v.commitDialog(c, o)
	v.prDialog(c, o)
	v.mergeDialog(c, o)
	v.discardDialog(c, o)
}

func centered(c *ui.Context, text string) {
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		ui.Text(c, text).TextColor(theme.Muted)
	})
}

// button is an action of the toolbar.
type button struct {
	action, label string
	kind          widget.ButtonKind
	svg           *ui.SVG
}

// toolbar is the branch, what it is compared with and how many files are
// uncommitted, then how it stands and the actions that fit it, wrapping
// onto more lines in a narrow panel.
func (v *View) toolbar(c *ui.Context, o Options) {
	st := v.status
	ui.Column(c).Shrink(0).FillWidth().Padding(6, 10, 8, 14).Gap(6).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		ui.Row(c).FillWidth().Height(30).Gap(8).Children(func() {
			ui.Icon(c, icon.Git).FontSize(13).TextColor(theme.Accent)
			branch := st.Branch
			if branch == "" {
				branch = "detached"
			}
			ui.Text(c, branch).FontSize(12.5).TextColor(theme.Text).SingleLine().Shrink(1)
			v.baseMenu(c, st)
			ui.Spacer(c)
			if v.busy != "" {
				widget.Spinner(c, 14)
				ui.Text(c, v.busy+"…").FontSize(11.5).TextColor(theme.TextSoft).SingleLine()
				return
			}
			if st.Uncommitted > 0 {
				widget.Tag(c, fmt.Sprintf("%d uncommitted", st.Uncommitted), theme.Warning)
			}
			if st.Worktree && v.loaded {
				more := widget.IconButton(c, icon.More, "More", false)
				v.moreMenu.OpenBelow(more, "more", 28)
				v.moreMenu.Show(c, more, "more", func(done func()) {
					if v.lastPRURL != "" && widget.MenuItem(c, icon.Globe, "Open the pull request", false) {
						o.OpenURL(v.lastPRURL)
						done()
					}
					if widget.MenuItem(c, icon.Trash, "Discard worktree…", true) {
						v.start(o, ActDiscard)
						done()
					}
				})
			}
			if widget.IconButton(c, icon.Sync, "Refresh", false).Clicked() {
				v.reload()
			}
		})
		if !v.loaded || v.busy != "" {
			return
		}
		actions := []button{
			{ActCommit, "Commit…", widget.Primary, icon.Check},
			{ActPush, "Push", widget.Secondary, icon.ArrowUp},
			{ActPR, "Pull request…", widget.Secondary, icon.Globe},
			{ActMerge, "Merge into " + st.Base, widget.Secondary, icon.Git},
		}
		sum := v.summary()
		offered := slices.ContainsFunc(actions, func(b button) bool { return v.can(b.action) })
		if sum == "" && !offered {
			return
		}
		ui.Row(c).FillWidth().Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			for _, a := range actions {
				if v.can(a.action) && widget.Button(c.Key(a.action), a.kind, a.svg, a.label, "").Clicked() {
					v.start(o, a.action)
				}
			}
			if sum != "" {
				ui.Text(c, sum).FontSize(11.5).TextColor(theme.Muted).SingleLine().Shrink(1)
			}
		})
	})
}

// Actions a view takes, which the app can also ask for (Ask), as from its
// command palette.
const (
	ActCommit  = "commit"
	ActPush    = "push"
	ActPR      = "pr"
	ActMerge   = "merge"
	ActDiscard = "discard"
)

// Ask takes action once the view has loaded, or says why it can't.
func (v *View) Ask(action string) { v.asked = action }

// can reports whether action fits where the branch stands.
func (v *View) can(action string) bool {
	st := v.status
	switch action {
	case ActCommit:
		return st.Uncommitted > 0
	case ActPush:
		return st.Remote != "" && st.Branch != "" && (st.Upstream == "" || st.Unpushed > 0)
	case ActPR:
		return st.HasGH && st.Remote != "" && st.Base != "" && st.Ahead > 0
	case ActMerge:
		return st.CanMerge && st.Ahead > 0 && st.Uncommitted == 0
	case ActDiscard:
		return st.Worktree
	}
	return false
}

// whyNot says why action doesn't fit now.
func (v *View) whyNot(action string) string {
	st := v.status
	switch action {
	case ActCommit:
		return "Nothing to commit."
	case ActPush:
		if st.Remote == "" {
			return "The repository has no remote to push to."
		}
		return "Nothing to push."
	case ActPR:
		switch {
		case !st.HasGH:
			return "Opening a pull request needs the GitHub CLI (gh) on this host."
		case st.Remote == "":
			return "The repository has no remote."
		case st.Base == "":
			return "Pick a branch to compare with first."
		}
		return st.Branch + " has no commits " + st.Base + " doesn't."
	case ActMerge:
		switch {
		case st.ProjectDirty:
			return "The project folder has uncommitted changes."
		case !st.CanMerge:
			return "Only a worktree can be merged, into the branch the project folder has checked out."
		case st.Uncommitted > 0:
			return "Commit the changes before merging them."
		}
		return st.Branch + " has nothing " + st.Base + " doesn't."
	case ActDiscard:
		return "Only a session's own worktree can be discarded."
	}
	return ""
}

// start takes action: a form for those that need one or a confirmation,
// at once for a push.
func (v *View) start(o Options, action string) {
	if !v.can(action) {
		v.err = v.whyNot(action)
		return
	}
	switch action {
	case ActCommit:
		v.commit = &commitForm{open: true}
	case ActPush:
		branch := v.status.Branch
		v.run(o, "Pushing", func(cl *client.Client) error {
			_, err := cl.Push(o.Session.ID, v.base)
			return err
		}, func() { o.Notify("Pushed " + branch) })
	case ActPR:
		v.pr = &prForm{open: true}
	case ActMerge:
		v.merging = true
	case ActDiscard:
		v.discard = &discardForm{open: true, deleteBranch: true}
	}
}

// summary is how the branch stands against its base and its remote.
func (v *View) summary() string {
	st := v.status
	var parts []string
	if st.Base != "" && (st.Ahead > 0 || st.Behind > 0) {
		parts = append(parts, fmt.Sprintf("%d ahead, %d behind %s", st.Ahead, st.Behind, st.Base))
	}
	switch {
	case st.Remote == "":
	case st.Upstream == "" && st.Ahead > 0:
		parts = append(parts, "not pushed")
	case st.Unpushed > 0:
		parts = append(parts, fmt.Sprintf("%d to push", st.Unpushed))
	}
	if v.diff.Truncated {
		parts = append(parts, "more files than shown")
	}
	return strings.Join(parts, " · ")
}

// baseMenu picks what the changes are compared with.
func (v *View) baseMenu(c *ui.Context, st proto.ReviewStatus) {
	label := "since last commit"
	if st.Base != "" {
		label = "vs " + st.Base
	}
	b := ui.ButtonBase(c).Height(24).PaddingX(7).Gap(4).Radius(theme.RadiusControl - 2).Cursor(ui.CursorPointer).
		Label("Compare " + label).Shrink(0)
	if b.Hovered() {
		b.Background(theme.Hover)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(theme.Muted).SingleLine()
		ui.Icon(c, icon.ChevronDown).FontSize(10).TextColor(theme.Muted)
	})
	v.baseMenuState.OpenBelow(b, "base", 24)
	v.baseMenuState.Show(c, b, "base", func(done func()) {
		if widget.MenuItem(c, icon.Check, "Since the last commit", false) {
			v.setBase(noBase)
			done()
		}
		for _, base := range st.Bases {
			if widget.MenuItem(c, icon.Git, "Against "+base, false) {
				v.setBase(base)
				done()
			}
		}
	})
}

// noBase asks for the changes since the last commit. The daemon takes an
// empty base for the session's default, so the last commit is "HEAD".
const noBase = "HEAD"

func (v *View) commitDialog(c *ui.Context, o Options) {
	f := v.commit
	if f == nil {
		return
	}
	submit := false
	widget.Dialog(c, &f.open, 480, 14, func() {
		widget.DialogHeader(c, "Commit changes", fmt.Sprintf("Commits all %d changed %s in %s, new files included.",
			v.status.Uncommitted, textfmt.Plural(v.status.Uncommitted, "file", "files"), v.status.Branch), false)
		in := widget.Input(c, &f.message, "Commit message").FillWidth().AutoFocus()
		submit = in.Submitted()
		widget.DialogFooter(c, "", func() {
			if widget.GhostButton(c, "Cancel").Clicked() {
				f.open = false
			}
			if widget.PrimaryButton(c, icon.Check, "Commit", "").Clicked() {
				submit = true
			}
		})
	})
	if submit && strings.TrimSpace(f.message) != "" {
		msg := f.message
		f.open = false
		v.run(o, "Committing", func(cl *client.Client) error {
			_, err := cl.Commit(o.Session.ID, msg)
			return err
		}, func() { o.Notify("Committed: " + textfmt.FirstLine(msg, 60)) })
	}
	if !f.open {
		v.commit = nil
	}
}

func (v *View) prDialog(c *ui.Context, o Options) {
	f := v.pr
	if f == nil {
		return
	}
	submit := false
	st := v.status
	widget.Dialog(c, &f.open, 520, 14, func() {
		widget.DialogHeader(c, "Open a pull request",
			fmt.Sprintf("Pushes %s and asks GitHub to merge it into %s, with gh on this host.", st.Branch, st.Base), false)
		widget.Field(c, "Title", func() {
			widget.Input(c, &f.title, "From the commits when empty").FillWidth().AutoFocus()
		})
		widget.Field(c, "Description", func() {
			widget.Input(c, &f.body, "Optional").FillWidth()
		})
		widget.DialogFooter(c, "", func() {
			if widget.GhostButton(c, "Cancel").Clicked() {
				f.open = false
			}
			if widget.PrimaryButton(c, icon.Globe, "Open pull request", "").Clicked() {
				submit = true
			}
		})
	})
	if submit {
		p := proto.PRParams{Session: o.Session.ID, Base: st.Base, Title: f.title, Body: f.body}
		f.open = false
		var url string
		v.run(o, "Opening a pull request", func(cl *client.Client) error {
			pr, err := cl.PullRequest(p)
			url = pr.URL
			return err
		}, func() {
			v.lastPRURL = url
			o.Notify("Opened " + url)
			o.OpenURL(url)
		})
	}
	if !f.open {
		v.pr = nil
	}
}

func (v *View) mergeDialog(c *ui.Context, o Options) {
	if !v.merging {
		return
	}
	st := v.status
	msg := fmt.Sprintf("Merges the %d %s of %s into %s in the project folder. A merge that conflicts is undone.",
		st.Ahead, textfmt.Plural(st.Ahead, "commit", "commits"), st.Branch, st.Base)
	switch ui.AlertDialog(c, &v.merging, "Merge "+st.Branch+" into "+st.Base+"?", msg, "Cancel", "Merge") {
	case 1:
		v.merging = false
		v.run(o, "Merging", func(cl *client.Client) error {
			_, err := cl.Merge(o.Session.ID, v.base)
			return err
		}, func() { o.Notify("Merged " + st.Branch + " into " + st.Base) })
	case 0, ui.AlertDismissed:
		v.merging = false
	}
}

func (v *View) discardDialog(c *ui.Context, o Options) {
	f := v.discard
	if f == nil {
		return
	}
	submit := false
	st := v.status
	widget.Dialog(c, &f.open, 480, 14, func() {
		body := "Ends the sessions in this worktree and deletes its folder."
		if st.Uncommitted > 0 {
			body += fmt.Sprintf(" Its %d uncommitted %s are lost.", st.Uncommitted, textfmt.Plural(st.Uncommitted, "change", "changes"))
		}
		widget.DialogHeader(c, "Discard the worktree of "+st.Branch+"?", body, false)
		widget.Checkbox(c, &f.deleteBranch, "Also delete the branch "+st.Branch)
		widget.DialogFooter(c, "", func() {
			if widget.GhostButton(c, "Cancel").Clicked() {
				f.open = false
			}
			if widget.DangerButton(c, "Discard").Clicked() {
				submit = true
			}
		})
	})
	if submit {
		del := f.deleteBranch
		f.open = false
		v.run(o, "Discarding", func(cl *client.Client) error {
			return cl.Discard(o.Session.ID, del)
		}, func() { o.Notify("Discarded the worktree of " + st.Branch) })
	}
	if !f.open {
		v.discard = nil
	}
}
