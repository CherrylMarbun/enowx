package proto

// Unslop: a mockup's design read for what makes it look made by AI
// without a designer's care (core/unslop), found by passes in the order
// they're best fixed, and, asked to start, the canvas's agent redesigning
// a copy of it pass by pass.
const (
	MethodCanvasUnslop = "canvas.unslop" // CanvasUnslopParams → CanvasUnslop
)

// CanvasUnslopParams reads design Frame of canvas ID (its first when
// empty). With Start the design is copied, the copy read, and Ask is what
// to tell the canvas's agent to redesign it.
type CanvasUnslopParams struct {
	ID    string `json:"id"`
	Frame string `json:"frame,omitempty"`
	Start bool   `json:"start,omitempty"`
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
// few of them, Fix how to fix it.
type UnslopFinding struct {
	Rule     string   `json:"rule"`
	Pass     string   `json:"pass"`
	Title    string   `json:"title"`
	Severity string   `json:"severity"` // high, medium, low
	Count    int      `json:"count"`
	Examples []string `json:"examples,omitempty"`
	Fix      string   `json:"fix"`
}

// Severities of an UnslopFinding.
const (
	UnslopHigh   = "high"
	UnslopMedium = "medium"
	UnslopLow    = "low"
)
