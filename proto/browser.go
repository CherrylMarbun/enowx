package proto

// Agents drive a session's Browser from the command line (enowx browser):
// the daemon hands each action to the app that serves Browsers, which does
// it in the session's web view and replies.
const (
	MethodBrowserAct = "browser.act" // BrowserAction → BrowserResult
	// MethodBrowserServe tells the daemon this connection (the app) does
	// browser.act's actions; the latest to say so does them.
	MethodBrowserServe = "browser.serve"
	MethodBrowserReply = "browser.reply" // BrowserReply
	// EventBrowserRequest asks the serving app to do an action:
	// BrowserRequest.
	EventBrowserRequest = "browser.request"
)

// Browser actions.
const (
	BrowserOpen       = "open"       // load URL
	BrowserSnapshot   = "snapshot"   // the page as text, its controls numbered
	BrowserClick      = "click"      // click Ref
	BrowserFill       = "fill"       // type Text into Ref
	BrowserScreenshot = "screenshot" // a PNG of the page
)

// BrowserAction is what to do in session Session's Browser. Ref is a
// control's number from the last snapshot, or a CSS selector.
type BrowserAction struct {
	Session string `json:"session"`
	Action  string `json:"action"`
	URL     string `json:"url,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Text    string `json:"text,omitempty"`
}

// BrowserResult is where the page is after the action, and what it gave:
// a snapshot's Text, a screenshot's Image (PNG).
type BrowserResult struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
	Image []byte `json:"image,omitempty"`
}

// BrowserRequest is an action for the app to do, ID to reply with.
type BrowserRequest struct {
	ID string `json:"id"`
	BrowserAction
}

// BrowserReply is the app's answer to request ID.
type BrowserReply struct {
	ID     string        `json:"id"`
	Result BrowserResult `json:"result"`
	Error  string        `json:"error,omitempty"`
}
