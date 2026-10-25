package proto

import "time"

// Canvases: mockups of UIs (HTML and CSS), wireframes, diagrams and free
// boards on a host, which the user and agents design together. An agent's
// changes reach every app as it makes them, an element at a time.
const (
	MethodCanvasList    = "canvas.list"    // → []CanvasInfo
	MethodCanvasGet     = "canvas.get"     // CanvasRef → Canvas
	MethodCanvasCreate  = "canvas.create"  // CanvasCreateParams → Canvas
	MethodCanvasDelete  = "canvas.delete"  // CanvasRef
	MethodCanvasEdit    = "canvas.edit"    // CanvasEdit → Canvas
	MethodCanvasHistory = "canvas.history" // CanvasRef → CanvasHistory
	MethodCanvasVersion = "canvas.version" // CanvasVersionRef → Canvas (as it was then)
	MethodCanvasUndo    = "canvas.undo"    // CanvasRef → Canvas
	MethodCanvasRedo    = "canvas.redo"    // CanvasRef → Canvas
	MethodCanvasRestore = "canvas.restore" // CanvasVersionRef → Canvas (a new version, as it was then)
	// MethodCanvasComments and MethodCanvasComment read and change the
	// comments on a canvas's elements.
	MethodCanvasComments = "canvas.comments" // CanvasRef → []CanvasComment
	MethodCanvasComment  = "canvas.comment"  // CanvasCommentParams → []CanvasComment
	// MethodCanvasControl pauses an agent designing a canvas, lets it go
	// on, or stops it.
	MethodCanvasControl  = "canvas.control"  // CanvasControl → CanvasInfo
	MethodCanvasAsset    = "canvas.asset"    // CanvasAsset → CanvasAsset (its Name, Data left out)
	MethodCanvasAssetGet = "canvas.assetGet" // CanvasAsset (ID, Name) → CanvasAsset with its Data
	// MethodCanvasRender renders a frame as a PNG or a PDF: a frame of
	// shapes alone by the daemon, one with HTML by the app serving
	// Browsers (EventCanvasRender), which answers with
	// MethodCanvasRenderReply.
	MethodCanvasRender      = "canvas.render"      // CanvasRenderParams → CanvasRendered
	MethodCanvasRenderReply = "canvas.renderReply" // CanvasRenderReply
)

// Canvas events.
const (
	// EventCanvasChanged carries a CanvasChange: a canvas after a change.
	EventCanvasChanged = "canvas.changed"
	// EventCanvasesChanged carries the full []CanvasInfo after a canvas
	// came, went or was renamed.
	EventCanvasesChanged = "canvas.listed"
	// EventCanvasRender asks the serving app to render: a CanvasRenderRequest.
	EventCanvasRender = "canvas.render"
)

// Kinds of canvas.
const (
	CanvasMockup    = "mockup"    // a UI in HTML and CSS
	CanvasWireframe = "wireframe" // a UI in boxes and text
	CanvasDiagram   = "diagram"   // a flow or an architecture
	CanvasBoard     = "board"     // shapes, text and images, free
)

// CanvasInfo is a canvas without its frames.
type CanvasInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	// Group is the group it is in, as a note's (NoteInfo.Group).
	Group string `json:"group,omitempty"`
	// Projects are the project folders it belongs to.
	Projects []string  `json:"projects,omitempty"`
	Version  int       `json:"version"` // the version shown, of History
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
	// Agent is the session of the agent that changed it last, at
	// AgentAt: an agent designing it now when that was moments ago.
	Agent   string    `json:"agent,omitempty"`
	AgentAt time.Time `json:"agentAt,omitzero"`
	// Paused: the user paused agents designing it, whose changes are
	// refused until resumed. The daemon keeps it while it runs.
	Paused bool `json:"paused,omitempty"`
}

// Canvas is a canvas with its frames: pages, or the same page at a
// device's sizes.
type Canvas struct {
	CanvasInfo
	Frames []CanvasFrame `json:"frames"`
}

// CanvasFrame is a page of a canvas, Width×Height: a mockup's HTML (its
// elements with id attributes, which agents change one at a time) and
// shapes above it.
type CanvasFrame struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	Background string          `json:"background,omitempty"` // #rrggbb
	HTML       string          `json:"html,omitempty"`
	Elements   []CanvasElement `json:"elements,omitempty"`
}

// Shapes of a frame's elements.
const (
	ShapeRect    = "rect"
	ShapeEllipse = "ellipse"
	ShapeDiamond = "diamond"
	ShapeText    = "text"
	ShapeSticky  = "sticky" // a note on a board
	ShapeLine    = "line"
	ShapeArrow   = "arrow"
	ShapeImage   = "image"
)

// CanvasElement is a shape of a frame at X, Y, W×H. A line or an arrow
// goes from element From to element To, or from (X, Y) to (X+W, Y+H)
// without them. Src is an image's asset (CanvasAsset.Name) or URL.
type CanvasElement struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	W           float64 `json:"w"`
	H           float64 `json:"h"`
	Text        string  `json:"text,omitempty"`
	Fill        string  `json:"fill,omitempty"`   // #rrggbb
	Stroke      string  `json:"stroke,omitempty"` // #rrggbb
	StrokeWidth float64 `json:"strokeWidth,omitempty"`
	Color       string  `json:"color,omitempty"` // the text's
	FontSize    float64 `json:"fontSize,omitempty"`
	Bold        bool    `json:"bold,omitempty"`
	Align       string  `json:"align,omitempty"` // left, center, right
	Radius      float64 `json:"radius,omitempty"`
	From        string  `json:"from,omitempty"`
	To          string  `json:"to,omitempty"`
	Src         string  `json:"src,omitempty"`
}

// CanvasRef names a canvas.
type CanvasRef struct {
	ID string `json:"id"`
}

// CanvasCreateParams makes a canvas of Kind, with Frames ("" sizes take
// the kind's) or one frame of the kind's.
type CanvasCreateParams struct {
	Title    string        `json:"title"`
	Kind     string        `json:"kind,omitempty"`
	Group    string        `json:"group,omitempty"`
	Projects []string      `json:"projects,omitempty"`
	Frames   []CanvasFrame `json:"frames,omitempty"`
}

// Edits of a canvas (CanvasEdit.Op).
const (
	CanvasOpMeta        = "meta"         // Title, Kind, Group, Projects (those set)
	CanvasOpFrameAdd    = "frame.add"    // FrameInfo
	CanvasOpFrameSet    = "frame.set"    // FrameInfo: Frame's name, size, background
	CanvasOpFrameRemove = "frame.remove" // Frame
	CanvasOpHTMLSet     = "html.set"     // HTML: Frame's whole
	// CanvasOpHTMLPatch puts HTML in place of Frame's element with id
	// Target, or, without one, at the end of element Into ("" for the
	// page's body).
	CanvasOpHTMLPatch  = "html.patch"
	CanvasOpHTMLRemove = "html.remove" // Target
	CanvasOpHTMLText   = "html.text"   // Target's text, as HTML
	CanvasOpElementSet = "el.set"      // Element: added, or put in place of the one with its ID
	CanvasOpElementDel = "el.remove"   // Target
)

// CanvasEdit is a change to canvas ID's frame Frame (its ID or name, ""
// for the first), made by the agent of Session, or by the user ("").
type CanvasEdit struct {
	ID        string         `json:"id"`
	Op        string         `json:"op"`
	Frame     string         `json:"frame,omitempty"`
	Target    string         `json:"target,omitempty"`
	Into      string         `json:"into,omitempty"`
	HTML      string         `json:"html,omitempty"`
	Element   *CanvasElement `json:"element,omitempty"`
	FrameInfo *CanvasFrame   `json:"frameInfo,omitempty"`
	Title     *string        `json:"title,omitempty"`
	Kind      *string        `json:"kind,omitempty"`
	Group     *string        `json:"group,omitempty"`
	Projects  *[]string      `json:"projects,omitempty"`
	Session   string         `json:"session,omitempty"`
}

// CanvasChange is a canvas after a change: what changed (Op, in Frame,
// element Element) and whose (Session, "" for the user's).
type CanvasChange struct {
	Canvas  Canvas `json:"canvas"`
	Op      string `json:"op,omitempty"`
	Frame   string `json:"frame,omitempty"`
	Element string `json:"element,omitempty"`
	Session string `json:"session,omitempty"`
}

// CanvasHistory is a canvas's versions, the oldest first; Head is the
// one shown, those after it what Redo brings back.
type CanvasHistory struct {
	Head     int             `json:"head"`
	Versions []CanvasVersion `json:"versions"`
}

// CanvasVersion is a version: what made it.
type CanvasVersion struct {
	N       int       `json:"n"`
	Time    time.Time `json:"time"`
	Op      string    `json:"op,omitempty"`
	Frame   string    `json:"frame,omitempty"`
	Element string    `json:"element,omitempty"`
	Session string    `json:"session,omitempty"`
}

// CanvasVersionRef names version N of canvas ID.
type CanvasVersionRef struct {
	ID string `json:"id"`
	N  int    `json:"n"`
}

// CanvasComment is a comment on a canvas, on element Element of frame
// Frame (an HTML id or a shape's), or the whole frame.
type CanvasComment struct {
	ID       string    `json:"id"`
	Frame    string    `json:"frame,omitempty"`
	Element  string    `json:"element,omitempty"`
	Text     string    `json:"text"`
	Author   string    `json:"author,omitempty"` // "" for the user; an agent's session
	Created  time.Time `json:"created"`
	Resolved bool      `json:"resolved,omitempty"`
}

// Comment changes (CanvasCommentParams.Op).
const (
	CanvasCommentAdd     = "add"
	CanvasCommentResolve = "resolve"
	CanvasCommentReopen  = "reopen"
	CanvasCommentDelete  = "delete"
)

// CanvasCommentParams adds Comment to canvas ID, or resolves, reopens or
// deletes the one with its ID.
type CanvasCommentParams struct {
	ID      string        `json:"id"`
	Op      string        `json:"op"`
	Comment CanvasComment `json:"comment"`
}

// Agents and the user designing a canvas (CanvasControl.Action).
const (
	CanvasPause  = "pause"
	CanvasResume = "resume"
	CanvasStop   = "stop" // interrupts the agent designing, and pauses
	// CanvasOpen shows the canvas in the apps' Canvas page.
	CanvasOpen = "open"
)

// CanvasControl pauses, resumes or stops the agents designing canvas ID,
// or shows it in the apps.
type CanvasControl struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

// CanvasAsset is a file a canvas shows, as an image: Name, under the
// canvas, and its Data.
type CanvasAsset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Data []byte `json:"data,omitempty"`
}

// Render formats: a picture, a PDF, a frame's shapes as SVG, or a page
// to open anywhere, read-only (the whole canvas without a frame named).
const (
	RenderPNG  = "png"
	RenderPDF  = "pdf"
	RenderSVG  = "svg"
	RenderHTML = "html"
)

// CanvasRenderParams renders canvas ID's frame Frame (its ID or name; a
// device, "desktop", "tablet" or "mobile", the first frame at its width,
// "" the first, or for HTML the whole canvas) as Format.
type CanvasRenderParams struct {
	ID     string `json:"id"`
	Frame  string `json:"frame,omitempty"`
	Format string `json:"format,omitempty"`
}

// CanvasRendered is a frame rendered: by the daemon ("daemon") or the app
// ("app").
type CanvasRendered struct {
	Data   []byte `json:"data"`
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	By     string `json:"by"`
}

// CanvasRenderRequest asks the app to render HTML, a page of Width×Height,
// as Format; it answers request ID with MethodCanvasRenderReply.
type CanvasRenderRequest struct {
	ID     string `json:"id"`
	HTML   string `json:"html"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
}

// CanvasRenderReply is the app's render for request ID.
type CanvasRenderReply struct {
	ID    string `json:"id"`
	Data  []byte `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}
