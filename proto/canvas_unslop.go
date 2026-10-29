package proto

// Unslop: a mockup's design read for what makes it look made by AI
// without a designer's care (core/unslop), found by passes in the order
// they're best fixed, and, asked to start, the canvas's agent redesigning
// a copy of it pass by pass.
const (
	MethodCanvasUnslop = "canvas.unslop" // CanvasUnslopParams → CanvasUnslop
)

// EventCanvasUnslop carries a CanvasUnslopRun as it goes: started, a pass
// begun, the design read again, done.
const EventCanvasUnslop = "canvas.unslop"

// CanvasUnslopParams reads design Frame of canvas ID (its first when
// empty). With Start the design is copied, the copy read, and Ask is what
// to tell the canvas's agent to redesign it. Pass, by the agent
// redesigning, says the pass it begins (its number or name; "done" when
// all are), the design read again with it; with Region, an agent of that
// region says it. Regions, from the redesign's agent (Session), fan the
// rest out: an agent each, as tabs of Session. Status answers the run
// alone, the design not read.
type CanvasUnslopParams struct {
	ID      string   `json:"id"`
	Frame   string   `json:"frame,omitempty"`
	Start   bool     `json:"start,omitempty"`
	Pass    string   `json:"pass,omitempty"`
	Region  string   `json:"region,omitempty"`
	Regions []string `json:"regions,omitempty"`
	Session string   `json:"session,omitempty"`
	Status  bool     `json:"status,omitempty"`
}

// UnslopRegion is a part of a design an agent redesigns, at once with
// others: element ID and all in it, the only part its Session may change;
// the pass it is at, and whether it is done.
type UnslopRegion struct {
	ID      string `json:"id"`
	Session string `json:"session"`
	Agent   string `json:"agent,omitempty"`
	Pass    string `json:"pass,omitempty"`
	Done    bool   `json:"done,omitempty"`
}

// CanvasUnslopRun is a redesign of canvas ID's design Frame as it goes, for
// the app to show: the score it started from and the latest, the pass
// under way (a name of core/unslop's passes), those done, the latest
// findings, and whether it is Done. The daemon keeps it while it runs.
type CanvasUnslopRun struct {
	ID       string          `json:"id"`
	Frame    string          `json:"frame"`
	From     int             `json:"from"`
	Score    int             `json:"score"`
	Pass     string          `json:"pass,omitempty"`
	Passes   []string        `json:"passes"` // every pass, in order
	Finished []string        `json:"finished,omitempty"`
	Findings []UnslopFinding `json:"findings"`
	Scans    int             `json:"scans"` // times read, the first 1
	// Theme is the original's, dark or light ("" unknown): the redesign
	// keeps it.
	Theme string `json:"theme,omitempty"`
	// Regions are the parts agents redesign at once, once fanned out.
	Regions []UnslopRegion `json:"regions,omitempty"`
	Done    bool           `json:"done,omitempty"`
}

// CanvasUnslop is what reading a design found: Score from 100 (nothing
// found) down, and the findings by pass. Frame is the design read (the
// copy, when started); Ask what to tell the canvas's agent.
type CanvasUnslop struct {
	Frame    string          `json:"frame"`
	Score    int             `json:"score"`
	Findings []UnslopFinding `json:"findings"`
	Ask      string          `json:"ask,omitempty"`
	// Run is the redesign under way it is part of, if any.
	Run *CanvasUnslopRun `json:"run,omitempty"`
}

// UnslopFinding is a tell found in a design: Rule its ID, Pass the pass
// fixing it ("01 Spacing"), Title what it is, Count how often, Examples a
// few of them, Fix how to fix it, Targets where.
type UnslopFinding struct {
	Rule     string   `json:"rule"`
	Pass     string   `json:"pass"`
	Title    string   `json:"title"`
	Severity string   `json:"severity"` // high, medium, low
	Count    int      `json:"count"`
	Examples []string `json:"examples,omitempty"`
	Fix      string   `json:"fix"`
	// Targets are the design's elements it is about, as CSS selectors.
	Targets []string `json:"targets,omitempty"`
}

// Severities of an UnslopFinding.
const (
	UnslopHigh   = "high"
	UnslopMedium = "medium"
	UnslopLow    = "low"
)
