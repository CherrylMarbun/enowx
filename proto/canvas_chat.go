package proto

import "time"

// The Canvas page's agent: a session (SessionCreateParams.CanvasFor) the
// user talks to beside a canvas, whose changes show live. The daemon keeps
// what was asked of it, with the canvas's version when it was, so a turn's
// changes are kept or undone, and which agent and mode the canvas uses.
const (
	MethodCanvasChat     = "canvas.chat"     // CanvasChatParams → CanvasChatLog
	MethodCanvasChatLog  = "canvas.chatLog"  // CanvasRef → CanvasChatLog
	MethodCanvasChatMark = "canvas.chatMark" // CanvasChatMark → CanvasChatLog
)

// EventCanvasChat carries a CanvasChatLog after a change.
const EventCanvasChat = "canvas.chat"

// EnvCanvasChat, set in a Canvas page agent's environment, leaves its
// enowx-app mcp with the canvas tools alone.
const EnvCanvasChat = "ENOWX_CANVAS_CHAT"

// Modes of the Canvas page's agent: its changes shown for the user to keep
// or undo after each message, or made with nothing asked.
const (
	CanvasChatReview = "review"
	CanvasChatDirect = "direct"
)

// CanvasChatLog is canvas ID's conversation with its agent: the agent and
// mode picked for it, and the messages sent, the oldest first.
type CanvasChatLog struct {
	ID       string            `json:"id"`
	Agent    string            `json:"agent,omitempty"`
	Mode     string            `json:"mode,omitempty"`
	Messages []CanvasChatEntry `json:"messages"`
}

// States of a message's changes.
const (
	CanvasChatKept   = "kept"
	CanvasChatUndone = "undone"
)

// CanvasChatEntry is a message sent to the agent: Version is the canvas's
// when it was sent, which undoing its changes goes back to; State says
// whether the user kept or undid them.
type CanvasChatEntry struct {
	N       int       `json:"n"`
	Time    time.Time `json:"time"`
	Text    string    `json:"text"`
	Files   []string  `json:"files,omitempty"`
	Version int       `json:"version"`
	Session string    `json:"session,omitempty"`
	State   string    `json:"state,omitempty"`
}

// CanvasChatParams records message Text (with Files attached) sent to
// canvas ID's agent in Session; with no Text, it sets the canvas's Agent
// or Mode alone.
type CanvasChatParams struct {
	ID      string   `json:"id"`
	Text    string   `json:"text,omitempty"`
	Files   []string `json:"files,omitempty"`
	Session string   `json:"session,omitempty"`
	Agent   string   `json:"agent,omitempty"`
	Mode    string   `json:"mode,omitempty"`
}

// CanvasChatMark says what became of message N's changes: kept or undone.
type CanvasChatMark struct {
	ID    string `json:"id"`
	N     int    `json:"n"`
	State string `json:"state"`
}
