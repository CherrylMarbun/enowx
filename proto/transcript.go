package proto

import "time"

// MethodSessionTranscript reads a session's conversation as the agent
// records it, for the chat view: TranscriptParams → Transcript.
const MethodSessionTranscript = "session.transcript"

// Kinds of transcript entries.
const (
	TranscriptUser      = "user"
	TranscriptAssistant = "assistant"
	TranscriptThinking  = "thinking"
	TranscriptTool      = "tool"
	TranscriptNotice    = "notice"
)

// TranscriptParams asks for the entries changed after revision Since.
type TranscriptParams struct {
	ID    string `json:"id"`
	Since int64  `json:"since,omitempty"`
}

// Transcript is a session's conversation, or why there is none.
type Transcript struct {
	Supported bool   `json:"supported"`        // the agent keeps a transcript enowX reads
	Reason    string `json:"reason,omitempty"` // why not, or why nothing shows yet
	// Entries are those added or changed after Since, by Index.
	Entries []TranscriptEntry `json:"entries,omitempty"`
	Rev     int64             `json:"rev"`   // the latest revision
	Reset   bool              `json:"reset"` // the transcript started over: drop what you have
}

// TranscriptEntry is one message, thought or tool call.
type TranscriptEntry struct {
	Index int       `json:"index"`
	Rev   int64     `json:"rev"`
	Kind  string    `json:"kind"`
	Time  time.Time `json:"time,omitzero"`
	Text  string    `json:"text,omitempty"` // the message, or a tool's name
	// Tool calls: what the call does, and its result once it came.
	Detail string `json:"detail,omitempty"`
	Result string `json:"result,omitempty"`
	Done   bool   `json:"done,omitempty"`
	Failed bool   `json:"failed,omitempty"`
}
