package proto

// Unslop in the Browser: a page of the session's project, running on
// localhost, read for what makes it look AI-made (core/unslop); started,
// the session's agent redesigns the project's code pass by pass, the page
// reloading as it goes.
const (
	MethodBrowserUnslop = "browser.unslop" // BrowserUnslopParams → CanvasUnslop (Frame: the page's URL)
	// EventBrowserUnslop carries the run as it goes: a CanvasUnslopRun
	// whose ID is the session's.
	EventBrowserUnslop = "browser.unslop"
)

// BrowserUnslopParams reads the page shown in session Session's Browser.
// With Start a run begins and Ask is what to tell the session's agent;
// Pass, by the agent, says the pass it begins (its number, or "done"),
// the page read again with it. Status answers the run alone.
type BrowserUnslopParams struct {
	Session string `json:"session"`
	Start   bool   `json:"start,omitempty"`
	Pass    string `json:"pass,omitempty"`
	Status  bool   `json:"status,omitempty"`
}
