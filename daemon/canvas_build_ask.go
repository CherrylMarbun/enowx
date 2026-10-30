package daemon

import (
	"fmt"
	"strings"

	"github.com/enowx/enowx/core/designbrief"
	"github.com/enowx/enowx/core/designcompare"
	"github.com/enowx/enowx/proto"
)

// What the UI builder's agents are told: the lead, from the brief to a
// finished design; each direction's fresh agent, from the brief alone.

// sources are where directions grow from, one each, so several never
// start from the same kind of thing.
var sources = []string{
	"a material and the place it is used in (ruled paper and a workshop bench, frosted glass and a lab, enamel and a train station…), chosen from the product's own world",
	"a scene: one time, place and light where the product's users are, seen precisely",
	"the visual language of another field the product touches (signage, print editorial, instruments, maps, packaging, broadcast graphics…): its grammar, not its decoration",
	"one convention of the product's category, broken on purpose and kept usable",
}

// buildAsk is what the canvas's agent is told when the user starts a
// build.
func buildAsk(cv proto.Canvas, p proto.CanvasBuildParams) string {
	at := fmt.Sprintf("canvas %q", cv.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "Build a UI on this canvas (%s): %s\n", at, nonEmpty(p.What, "recreate the screenshot the user gave"))
	fmt.Fprintf(&b, "Devices: %s. The user watches every step on the canvas. Say where you are with canvas_build (%s, stage …) as you go: brief, directions, building, compare, refining.\n\n", strings.Join(p.Devices, " and "), at)

	b.WriteString("1. What to deliver: if it is unclear which page, which device, or whether it is a new design or a change, ask the user once, all at once, and stop; otherwise start at once, saying what you assumed.\n")
	brief := "Write the brief with canvas_brief (" + at + ", brief): Markdown, a ## section a field (Product, Users, Tone, First screen, Peers, Remove, Fixed)."
	if cv.Brief != nil {
		brief = "The canvas has a brief (canvas_brief, " + at + "): read it, and write it again where this build changes it."
	}
	fmt.Fprintf(&b, "2. Understand the product before designing: its category, who uses it and for what. Look at two or three products of its kind with the most distinctive looks, not the most famous (web search, their pages): what to keep (what users expect), what's the category's habit to break, what to do differently. Turn words like clean, premium or modern into decisions (calm or lively, dense or airy, polished or raw; then type, space, how much of a screen carries color). Decide what the first screen is for (the work itself for apps, dashboards, shops; a designed opening for pages that persuade), its one focal point and one main action, and list what to remove. %s The user can edit it in the canvas's Brief; read it again before each round.\n", brief)

	switch {
	case p.From == proto.BuildScreenshot:
		fmt.Fprintf(&b, "3. Recreate the screenshot (the canvas's picture %q, canvas_show lists it) as a mockup, faithfully: no new style. Note its size (a 2x picture is half that in CSS pixels) and make a design that size; match positions, proportions and type first, then borders, shadows and detail. After each round compare: canvas_match (%s, design N, image %q), and fix the cells that differ most first. Then make its phone layout as a design of its own. Improve or unslop it only if the user asks.\n", p.Image, at, p.Image)
	case p.From == proto.BuildProject:
		fmt.Fprintf(&b, "3. The project (%s) first: read its design system, tokens, components and conventions, and follow them. Recreate the page as it is now as a design named “Current”, the baseline. %s\n", p.Project, workAsk(p.Work, at))
	case p.From == proto.BuildChange:
		fmt.Fprintf(&b, "3. Change design %s: copy it first (canvas_new_design, from it), keep what the brief says is fixed, and change the copy.\n", p.Frame)
	}
	if p.Work != proto.WorkReview && p.From != proto.BuildScreenshot {
		if p.Directions > 1 {
			fmt.Fprintf(&b, "4. Directions: the user wants %d to compare. When the brief is written, canvas_build (%s, fanout %d): fresh agents each build one on a design of its own, from the brief alone, each from a different kind of source; the daemon checks they differ (of skeleton, type, palette and hero any two share at most one; their first screens squinted at must differ) and sends back the too-alike. Change nothing meanwhile and end your turn: you're told when they're built.\n", p.Directions, at, p.Directions)
		} else {
			b.WriteString("4. One direction: think of three, each from a different concrete source (a material and where it's used, a scene, another field's visual language, a convention broken on purpose), at least one from something only this product has; keep the one that fits the brief best. Write its card on a new design (canvas_new_design, then canvas_direction with the card).")
			if p.Confirm {
				b.WriteString(" Then tell the user the direction in three or four lines (how it feels, the first screen, the main action, type, color, the main picture) and why, and end your turn: build it once they agree.")
			}
			b.WriteString("\n")
		}
		b.WriteString("5. Build it in the order a designer would: the structure first, in grays (the first screen's focal point, main action and states), checked before any color or type; then the representative screen with its key states; then the rest. One memorable moment (a reaction to the main action, a success, work under way, an empty or error state), the rest quiet. Pictures made for it, kept with the canvas (canvas_image: an SVG illustration you draw, or a picture chosen from the web), never gray boxes. Content that reads real (1,238 not 1,000; real names; specific buttons); nothing saying “sample” or “placeholder”.\n")
		fmt.Fprintf(&b, "6. Compare: canvas_compare the designs, the current one first when there is one, and tell the user which you recommend and why; they pick in Compare, with what to keep and change. Refine the one picked around that; keep the others.\n")
	}
	fmt.Fprintf(&b, "7. Check and finish: look at the design at desktop and phone sizes (canvas_screenshot, frame desktop and mobile); then unslop it as the last pass: canvas_unslop (%s, design N) and fix what it finds that you wouldn't keep on purpose; take away once more (one focal point a screen, no copy, lines or boxes that don't help). Then canvas_build (%s, done, design N): a fresh agent reviews it independently, and you're told its verdict; below the bar, fix what it lists and say done again.\n", at, at)
	b.WriteString("8. When the user is happy: Make code (the canvas's toolbar) builds it in their project, its tokens and rules kept as the project's design file.")
	return b.String()
}

// workAsk is how each kind of work on a project's pages goes.
func workAsk(work, at string) string {
	switch work {
	case proto.WorkRefresh:
		return "A UI refresh: first tidy the noise (what repeats, explains the obvious, decorates without helping), then upgrade the layout. Brand assets stay; defaults nobody chose may go. Show the work as designs here before any code changes: the code changes only once the user picks."
	case proto.WorkFlow:
		return "A flow fix: count the steps and clicks of the flow now, write them down, then cut them, keeping the project's look. Show the new flow as designs, a screen each, with the counts before and after; the code changes only once the user picks."
	case proto.WorkFeature:
		return "A new feature: fit what is there, its look, components and patterns; nothing new where something existing serves. Show it as designs before any code changes."
	case proto.WorkReview:
		return "A review only: change nothing. On the Current design, leave a comment on each element worth changing (canvas_comment, element …): what and why, the worst first, at most ten. Then canvas_build (" + at + ", done, design 1) and tell the user the three that matter most."
	}
	return ""
}

// directionAsk is what a direction's fresh agent is told: the brief alone,
// its source, its design.
func directionAsk(cv proto.Canvas, f proto.CanvasFrame, n int, letter, source string, run proto.CanvasBuildRun) string {
	at := fmt.Sprintf("canvas %q", cv.ID)
	brief := ""
	if cv.Brief != nil {
		brief = designbrief.Markdown(*cv.Brief)
	}
	return fmt.Sprintf(`You design Direction %s of a UI, one of several agents each designing one at once, the user watching live. Yours is design %d (frame %q) of %s, and only it: changes to anything else are refused. You know only the brief; never look at the other directions.

What to build: %s. Devices: %s.
%s
Grow your direction from %s. Never from an adjective, and never from the category's usual look.

1. Write your card first: canvas_direction (%s, design %d, card), in this form:
%s
2. Build its structure in grays first (the first screen's focal point, main action and states), then the real design, on your design only (canvas_write or canvas_patch, frame %q). One memorable moment, the rest quiet. Pictures made for it in its own style, kept with the canvas (canvas_image: an SVG you draw, or one chosen from the web), never gray boxes; content that reads real.
3. Look at it at desktop and phone width (canvas_screenshot, frame %q) and fix what reads wrong.
4. Then canvas_build (%s, done). If you're told it is too close to another direction, change what you're told and say done again.`,
		letter, n, f.ID, at, nonEmpty(run.What, "see the brief"), strings.Join(run.Devices, " and "), brief, source, at, n, designcompare.CardFormat, f.ID, f.ID, at)
}

func nonEmpty(s, or string) string {
	if strings.TrimSpace(s) == "" {
		return or
	}
	return s
}
