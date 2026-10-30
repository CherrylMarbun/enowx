package proto

// An independent review of a design: when an unslop (or a design the UI
// builder makes) is done, a fresh agent that saw none of the work looks at
// its screenshots and brief, never its code, and scores it on a fixed
// rubric (core/designreview); below the bar, the design's agent fixes what
// it lists and the review runs again.
const (
	// MethodDesignReview is the reviewer's verdict: DesignReviewParams →
	// DesignReview.
	MethodDesignReview = "design.review"
)

// DesignReviewParams is reviewer session Session's verdict, as the
// rubric's Markdown (Score:, Issue:, Improve:, Keep:, Next:).
type DesignReviewParams struct {
	Session string `json:"session"`
	Text    string `json:"text"`
}

// DesignReview is a review of a design: Round from 1, its reviewer
// Session, and once in, Score from 1 to 10, Passed when it reached the
// bar, the Issues and Improvements to make (each "where: what → how to
// tell it's fixed"), what to Keep, and what the Next band needs.
type DesignReview struct {
	Round        int      `json:"round"`
	Session      string   `json:"session,omitempty"`
	Score        int      `json:"score,omitempty"`
	Passed       bool     `json:"passed,omitempty"`
	Issues       []string `json:"issues,omitempty"`
	Improvements []string `json:"improvements,omitempty"`
	Keep         string   `json:"keep,omitempty"`
	Next         string   `json:"next,omitempty"`
	Done         bool     `json:"done,omitempty"`
}
