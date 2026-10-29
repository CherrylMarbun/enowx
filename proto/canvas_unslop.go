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
// all are), the design read again with it.
type CanvasUnslopParams struct {
	ID    string `json:"id"`
	Frame string `json:"frame,omitempty"`
	Start bool   `json:"start,omitempty"`
	Pass  string `json:"pass,omitempty"`
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
	Done     bool            `json:"done,omitempty"`
}

// CanvasUnslop is what reading a design found: Score from 100 (nothing
// found) down, and the findings by pass. Frame is the design read (the
// copy, when started); Ask what to tell the canvas's agent.
type CanvasUnslop struct {
	Frame    string          `json:"frame"`
	Score    int             `json:"score"`
	Findings []UnslopFinding `json:"findings"`
	Ask      string          `json:"ask,omitempty"`
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
