package proto

import "time"

// A canvas shared by a link: the host serves a read-only copy of it to
// whoever has the link, on this device and the user's tailnet only, until
// the link is stopped. No link exists until the user makes one.
const (
	MethodCanvasShare  = "canvas.share"  // CanvasShareParams → CanvasShare (Stop: nothing)
	MethodCanvasShares = "canvas.shares" // → []CanvasShare
)

// CanvasShareParams makes canvas ID's link (its own when it has one), or
// stops it.
type CanvasShareParams struct {
	ID   string `json:"id"`
	Stop bool   `json:"stop,omitempty"`
}

// CanvasShare is a canvas's link: URLs on this device and on the tailnet,
// the same page.
type CanvasShare struct {
	Canvas  string    `json:"canvas"`
	Title   string    `json:"title"`
	Token   string    `json:"token"`
	URLs    []string  `json:"urls"`
	Created time.Time `json:"created"`
}
