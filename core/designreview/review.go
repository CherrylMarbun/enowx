// Package designreview is the independent review of a design: the rubric a
// fresh agent scores screenshots on, what it is told, and its verdict read
// back (proto.DesignReview).
package designreview

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Bar is the score a design passes at; MaxRounds the reviews a design gets
// before its agent stops and says what is left.
const (
	Bar       = 8
	MaxRounds = 3
)

// Rubric is how a reviewer judges, the same every time.
const Rubric = `How to judge (the bar is the best product of its kind, not the average):
- Vetoes, each the first issue and the score at most 7: real text unreadable at the size shown; the main picture still looks like a placeholder; swap the product's name for another product's and the first screen still works; after three seconds you can't say what the screen is about or where its main action is.
- Three seconds: look at each screenshot small, as a thumbnail, for three seconds. Say what it looks like (what kind of product, from when) and what you saw first. Dated, generic, a component library's or a template's look: at most 7.
- Then judge: hierarchy (one focal point and one main action a screen, the rest quiet), type (clear roles, readable sizes and line lengths), color (each color one job; state never by color alone), space (same relation, same gap; groups tighter inside than between; few left edges), usability (the main path clear, states visible), mobile (it reflows; nothing overflows or shrinks to tiny), distinctness (what makes it this product's and not any product's of its kind; where the brief names a world and a signature: the world in its accent, charts and pictures, the surfaces still quiet, and the signature where it belongs and nowhere else).
- The tells of a design made by default: a split hero with text left and a picture right for no reason; cards and borders at every level; a colored stripe on one side of boxes; gradients, glows or emoji doing no work; a small label over every heading; the category's usual colors with no reason of the product's; copy that is vague or metaphorical; the logo and name taking room on every screen.
- Then what would lift it, saying for each of the three where it stands, or "none" and why: polish (at twice the size: spacing, alignment of icons and text, shadows and strokes, states alike; count the left edges of headings and text blocks down the page: an ordered page shares two or three, five or six is a fault); design (name each screen's composition in one word: the same word twice in a row, or three times on a page, is a template repeating; every section boxed in a card is too); motion (static screenshots show none: say "motion not seen", never guess).
- Filler: text that balances the layout without pointing at an object, an action or data, and notes left on the screen ("sample", "placeholder"): name each, to delete. Every screen one focal point; name what else competes. Taking away must leave the focal point stronger, not the screen emptier.
- Write targets, not compositions: what the fixed screen shows and how to tell, never how many things to add. Anything you'd add names what goes to make room.
- Score 1 to 10: 10 nothing to fault and one move unseen in its category; 9 solid everywhere, one part best of its kind; 8 a direction of its own, the main path works, recognisable as a thumbnail, some medium issues; 7 a default first screen, a placeholder-like picture, or one place a person gets stuck; 5-6 a template's skeleton or several stuck places; 1-4 the main task fails, or text overlaps, is cut off or unreadable. Unsure between two, take the lower.`

// Format is how the reviewer writes its verdict, read back by Parse.
const Format = `Score: N (1 to 10)
Issue: where: what is wrong → how to tell it's fixed   (at most 5, the worst first; "Redo: …" first when the skeleton itself is the problem)
Improve: where: what would make it better → how to tell   (at most 5)
Keep: what works and must stay
Next: what the next band up needs`

// Existing is what a reviewer of work on a project's existing pages is
// told besides the rubric: its own rules first.
const Existing = `This is work on a project's existing pages: judge it against the project's own rules first (its colors, type sizes, radii and components; where it has none written, what most of its pages do). Name what departs from them and any component hand-made where one exists. Never suggest changing its brand color or typeface.`

// Ask is what a reviewer is told: what the design is for (the brief as
// Markdown), what it was asked to be (task), the screenshots to look at
// (files in its folder) and the command that takes its verdict.
func Ask(task, brief string, shots []string, submit string) string {
	return fmt.Sprintf(`You are an independent design reviewer. You saw none of the work and must not read its code or ask its agent: judge only what a person would see. The design: %s.

What it is for:
%s
The screenshots, in this folder: %s. Look at each one (read the image files) before judging.

%s

Write your verdict in exactly this form, nothing else, and send it with: %s, the verdict on stdin (a heredoc). Then stop.

%s`, task, brief, strings.Join(shots, ", "), Rubric, submit, Format)
}

// Parse reads a verdict written in Format.
func Parse(text string) (proto.DesignReview, error) {
	var r proto.DesignReview
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*• "))
		key, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "score":
			n, _ := strconv.Atoi(strings.TrimSpace(strings.Split(strings.Fields(v + " ")[0], "/")[0]))
			r.Score = n
		case "issue", "redo":
			if strings.EqualFold(strings.TrimSpace(key), "redo") {
				v = "Redo: " + v
			}
			if v != "" && len(r.Issues) < 5 {
				r.Issues = append(r.Issues, v)
			}
		case "improve":
			if v != "" && len(r.Improvements) < 5 {
				r.Improvements = append(r.Improvements, v)
			}
		case "keep":
			r.Keep = v
		case "next":
			r.Next = v
		}
	}
	if r.Score < 1 || r.Score > 10 {
		return r, errors.New("review: no score from 1 to 10 (a line \"Score: N\")")
	}
	r.Passed, r.Done = r.Score >= Bar, true
	return r, nil
}

// Told is what the design's agent is told of review r: passed, or what to
// fix and how to have it reviewed again (again), or that the rounds are
// spent.
func Told(r proto.DesignReview, again string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Independent review, round %d: %d/10", r.Round, r.Score)
	if r.Passed {
		b.WriteString(", it passes.")
		if r.Keep != "" {
			b.WriteString(" Keep: " + r.Keep + ".")
		}
		if len(r.Improvements) > 0 {
			b.WriteString(" Worth doing still: " + strings.Join(r.Improvements, "; ") + ".")
		}
		return b.String()
	}
	fmt.Fprintf(&b, ", under the bar of %d.", Bar)
	if r.Round >= MaxRounds {
		b.WriteString(" That was the last round: stop changing it, and tell the user what is still left:")
	} else {
		b.WriteString(" Fix these, in order, then " + again + " to have it reviewed again:")
	}
	for _, i := range r.Issues {
		b.WriteString("\n- " + i)
	}
	for _, i := range r.Improvements {
		b.WriteString("\n- " + i)
	}
	if r.Keep != "" {
		b.WriteString("\nKeep: " + r.Keep)
	}
	if r.Next != "" {
		b.WriteString("\nThe next band needs: " + r.Next)
	}
	return b.String()
}
