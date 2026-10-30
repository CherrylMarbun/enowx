package proto

// The UI builder: from a brief to a finished design on a canvas. The
// canvas's agent writes the brief, recommends one direction (or, asked to
// explore, has fresh agents build several at once, a design each, their
// distinctness checked by the daemon), the user compares and picks, and
// the chosen design is refined, unslopped and reviewed independently.
const (
	MethodCanvasBuild = "canvas.build" // CanvasBuildParams → CanvasBuild
	// MethodCanvasMatch compares a design with a reference picture (a
	// screenshot recreated): CanvasMatchParams → CanvasMatch.
	MethodCanvasMatch = "canvas.match"
	// EventCanvasBuild carries a CanvasBuildRun as it goes.
	EventCanvasBuild = "canvas.build"
)

// What a build starts from (CanvasBuildParams.From).
const (
	BuildNew        = "new"        // a new design
	BuildChange     = "change"     // a change to the canvas's design Frame
	BuildProject    = "project"    // a project's existing pages
	BuildScreenshot = "screenshot" // a picture to recreate, then improve if asked
)

// The work on a project's pages (CanvasBuildParams.Work).
const (
	WorkRefresh = "refresh" // tidy the noise, then upgrade the layout
	WorkFlow    = "flow"    // count the steps and clicks, then cut them
	WorkFeature = "feature" // a new feature fitting what is there
	WorkReview  = "review"  // what to change and why, nothing changed
)

// Stages of a build (CanvasBuildRun.Stage).
const (
	BuildBrief      = "brief"
	BuildDirections = "directions"
	BuildBuilding   = "building"
	BuildCompare    = "compare"
	BuildRefining   = "refining"
	BuildReviewing  = "reviewing"
	BuildDone       = "done"
)

// CanvasBuildParams starts or advances a build on canvas ID. Start, from
// the app's Build dialog, begins one: What in the user's words, From what,
// Frame the design to change, Project and Work for a project's pages,
// Image a canvas asset to recreate, Devices (desktop, mobile), Directions
// (1, or 2 to 4 to compare) and Confirm (wait for the user's OK of the
// direction); Ask is what to tell the canvas's agent. From the agent
// (Session): Stage says where it is, Fanout starts that many fresh agents
// on a direction each, Done ends a direction's agent's work (as itself)
// or the build (as the lead), which has it reviewed. Status answers the
// run alone.
type CanvasBuildParams struct {
	ID         string   `json:"id"`
	Start      bool     `json:"start,omitempty"`
	What       string   `json:"what,omitempty"`
	From       string   `json:"from,omitempty"`
	Frame      string   `json:"frame,omitempty"`
	Project    string   `json:"project,omitempty"`
	Work       string   `json:"work,omitempty"`
	Image      string   `json:"image,omitempty"`
	Devices    []string `json:"devices,omitempty"`
	Directions int      `json:"directions,omitempty"`
	Confirm    bool     `json:"confirm,omitempty"`
	Stage      string   `json:"stage,omitempty"`
	Fanout     int      `json:"fanout,omitempty"`
	Done       bool     `json:"done,omitempty"`
	Session    string   `json:"session,omitempty"`
	Status     bool     `json:"status,omitempty"`
}

// CanvasBuild is what a build call answers: what to tell the agent, and
// the run.
type CanvasBuild struct {
	Ask string          `json:"ask,omitempty"`
	Run *CanvasBuildRun `json:"run,omitempty"`
}

// CanvasBuildRun is a build under way on canvas ID: what was asked, its
// stage, the directions' designs and agents, how distinct each pair is,
// the lead agent, and the independent review once done.
type CanvasBuildRun struct {
	ID         string           `json:"id"`
	What       string           `json:"what"`
	From       string           `json:"from"`
	Work       string           `json:"work,omitempty"`
	Devices    []string         `json:"devices,omitempty"`
	Want       int              `json:"want"` // the directions asked for
	Stage      string           `json:"stage"`
	Directions []BuildDirection `json:"directions,omitempty"`
	Pairs      []DirectionPair  `json:"pairs,omitempty"`
	Lead       string           `json:"lead,omitempty"`
	Review     *DesignReview    `json:"review,omitempty"`
	Done       bool             `json:"done,omitempty"`
}

// BuildDirection is a direction's design Frame, built by agent Session
// (none when the lead builds it), Done once it said so.
type BuildDirection struct {
	Frame   string `json:"frame"`
	Session string `json:"session,omitempty"`
	Done    bool   `json:"done,omitempty"`
}

// DirectionPair is how two directions' designs differ: Shared what they
// have in common of skeleton, type, palette and hero (at most one may
// be), Alike how alike their first screens' light and dark shapes are
// (0 to 100, squinted at), Distinct a score from 0 (the same) to 100.
type DirectionPair struct {
	A        string   `json:"a"`
	B        string   `json:"b"`
	Shared   []string `json:"shared,omitempty"`
	Alike    int      `json:"alike"`
	Distinct int      `json:"distinct"`
}

// DesignDirection is a design's direction card: its north star in a
// sentence, the concrete Source it grew from (a material, a place, a
// scene, another field's visual language; never an adjective), its
// first-screen Skeleton, Type (families, weights, scale), Palette (hex,
// background first, then the accent and the rest), Hero (the main
// picture and where it comes from), Icons, Controls (radius, stroke,
// press, in a sentence), its one memorable Moment, and what it GivesUp.
type DesignDirection struct {
	NorthStar string   `json:"northStar"`
	Source    string   `json:"source"`
	Skeleton  string   `json:"skeleton"`
	Type      string   `json:"type"`
	Palette   []string `json:"palette,omitempty"`
	Hero      string   `json:"hero,omitempty"`
	Icons     string   `json:"icons,omitempty"`
	Controls  string   `json:"controls,omitempty"`
	Moment    string   `json:"moment,omitempty"`
	GivesUp   string   `json:"givesUp,omitempty"`
}

// CanvasMatchParams compares design Frame of canvas ID with Image, a
// canvas asset (the screenshot it recreates), at the picture's size.
type CanvasMatchParams struct {
	ID    string `json:"id"`
	Frame string `json:"frame"`
	Image string `json:"image"`
}

// CanvasMatch is how far a design is from its reference: Diff the share of
// pixels that differ (0 to 100), Cells the same for a 3×3 grid (top left
// first), Worst the cells differing most, named ("top left"…).
type CanvasMatch struct {
	Diff  float64    `json:"diff"`
	Cells [9]float64 `json:"cells"`
	Worst []string   `json:"worst,omitempty"`
	// Overlay is the design over the reference at half opacity, a PNG.
	Overlay []byte `json:"overlay,omitempty"`
}
