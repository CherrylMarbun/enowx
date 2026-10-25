package proto

import "time"

// Traffic is a host's HTTP interceptor (docs/plans/traffic.md): its
// daemon runs a proxy on loopback that apps of its sessions, browsers,
// desktop apps and Android devices send their HTTP through, records each
// exchange and applies the project's rules.
const (
	MethodTrafficStatus   = "traffic.status"    // → TrafficStatus
	MethodTrafficCA       = "traffic.ca"        // → TrafficCA
	MethodTrafficAttach   = "traffic.attach"    // TrafficAttachParams → TrafficTarget
	MethodTrafficDetach   = "traffic.detach"    // TrafficTargetRef
	MethodTrafficAttached = "traffic.attached"  // → []TrafficTarget
	MethodTrafficEnv      = "traffic.env"       // TrafficEnvParams → TrafficEnv
	MethodTrafficList     = "traffic.list"      // TrafficListParams → TrafficList
	MethodTrafficGet      = "traffic.get"       // TrafficRef → TrafficExchange
	MethodTrafficBody     = "traffic.body"      // TrafficBodyParams → TrafficBody
	MethodTrafficMessages = "traffic.messages"  // TrafficMessagesParams → []TrafficMessage
	MethodTrafficPin      = "traffic.pin"       // TrafficPinParams
	MethodTrafficClear    = "traffic.clear"     // TrafficClearParams
	MethodTrafficRules    = "traffic.rules"     // TrafficRulesParams → TrafficRules
	MethodTrafficRulesSet = "traffic.rules.set" // TrafficRules → TrafficRules
	MethodTrafficPaused   = "traffic.paused"    // → []TrafficPaused
	MethodTrafficResume   = "traffic.resume"    // TrafficResumeParams
	MethodTrafficSend     = "traffic.send"      // TrafficSendParams → TrafficSummary
	MethodTrafficExport   = "traffic.export"    // TrafficExportParams → TransferInfo (read with transfer.read)
	MethodTrafficImport   = "traffic.import"    // TrafficImportParams → TrafficList
	// traffic.reverse puts a reverse proxy in front of a session's port,
	// recorded as the session's Browser tool's traffic.
	MethodTrafficReverse = "traffic.reverse" // TrafficReverseParams → TrafficReverse
	// traffic.tunnel is a tunnel to a source's own port, for a device on
	// another host to send through.
	MethodTrafficTunnel = "traffic.tunnel" // TrafficTargetRef → TunnelInfo
	// traffic.mcp adds enowX's MCP server (enowx-app mcp: the traffic and
	// browser tools) to the host's agents.
	MethodTrafficMCP = "traffic.mcp" // → TrafficMCP
	// traffic.ca.new makes the host's CA anew: what trusted the old one
	// must be given the new.
	MethodTrafficCANew       = "traffic.ca.new"       // → TrafficCA
	MethodTrafficSettings    = "traffic.settings"     // → TrafficSettings
	MethodTrafficSetSettings = "traffic.settings.set" // TrafficSettings → TrafficSettings

	// EventTrafficExchange tells of an exchange started, changed or done:
	// a TrafficSummary.
	EventTrafficExchange = "traffic.exchange"
	// EventTrafficPaused tells that the paused exchanges changed: a
	// []TrafficPaused.
	EventTrafficPaused = "traffic.paused"
	// EventTrafficAttached tells that the targets attached changed: a
	// []TrafficTarget.
	EventTrafficAttached = "traffic.attached"
)

// TrafficStatus is the interceptor of a host.
type TrafficStatus struct {
	Port        int    `json:"port"` // on loopback, 0 while it isn't running
	Fingerprint string `json:"fingerprint"`
	Exchanges   int    `json:"exchanges"` // kept
	Bytes       int64  `json:"bytes"`     // of bodies kept
	Error       string `json:"error,omitempty"`
	// Browsers are the browsers found on the host, to attach.
	Browsers []TrafficProgram `json:"browsers,omitempty"`
	// Display says whether windows started there show through a virtual
	// display (a server without a screen).
	Display bool `json:"display,omitempty"`
}

// TrafficProgram is a browser found on a host.
type TrafficProgram struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// TrafficCA is the host's certificate authority, to trust where traffic
// is caught.
type TrafficCA struct {
	PEM         string `json:"pem"`
	Fingerprint string `json:"fingerprint"`
	SPKI        string `json:"spki"` // base64 SHA-256 of its public key
	File        string `json:"file"` // where it is on the host
	Bundle      string `json:"bundle"`
}

// Kinds of targets the interceptor can be attached to.
const (
	TargetSession  = "session"  // a session's run tabs and new tabs
	TargetTerminal = "terminal" // a new terminal tab, with the environment
	TargetShell    = "shell"    // an existing tab, the environment typed into its shell
	TargetBrowser  = "browser"  // a browser of the host, on a fresh profile
	TargetElectron = "electron" // an Electron app
	TargetWebView  = "webview"  // the Browser tool's web view
	TargetAndroid  = "android"  // an Android device
	TargetDocker   = "docker"   // containers started from a session
	TargetManual   = "manual"   // anything given the proxy's address by hand
)

// TrafficAttachParams attaches the interceptor to a target: Kind, and
// what it needs (a session, a tab, a browser's or app's path, a device).
type TrafficAttachParams struct {
	Kind    string `json:"kind"`
	Session string `json:"session,omitempty"`
	Tab     string `json:"tab,omitempty"`     // TargetShell: the tab's session id
	Program string `json:"program,omitempty"` // TargetBrowser, TargetElectron: its path or id
	Device  string `json:"device,omitempty"`  // TargetAndroid: adb's serial
	Agent   bool   `json:"agent,omitempty"`   // TargetSession: the agent's own traffic too
	Label   string `json:"label,omitempty"`
}

// TrafficTarget is a target attached: its traffic carries Source.
type TrafficTarget struct {
	Source   string    `json:"source"`
	Kind     string    `json:"kind"`
	Label    string    `json:"label"`
	Session  string    `json:"session,omitempty"`
	Tab      string    `json:"tab,omitempty"` // the tab made or used
	Device   string    `json:"device,omitempty"`
	Program  string    `json:"program,omitempty"`
	Proxy    string    `json:"proxy"` // http://source:token@127.0.0.1:port
	Attached time.Time `json:"attached"`
	Note     string    `json:"note,omitempty"` // what to know: an app ignoring proxies
}

// TrafficTargetRef names a target by its source. For traffic.tunnel,
// Port names one of the source's reverse proxies instead of its own port.
type TrafficTargetRef struct {
	Source string `json:"source"`
	Port   int    `json:"port,omitempty"`
}

// TrafficEnvParams asks for the environment that sends a shell's traffic
// through the interceptor, as source Source (a new manual one when "").
type TrafficEnvParams struct {
	Session string `json:"session,omitempty"`
	Source  string `json:"source,omitempty"`
}

// TrafficEnv is that environment, and the same as shell commands.
type TrafficEnv struct {
	Source string   `json:"source"`
	Env    []string `json:"env"`
	Shell  string   `json:"shell"` // export lines for sh
	Unset  string   `json:"unset"` // what undoes them
}

// TrafficSummary is an exchange as a list shows it.
type TrafficSummary struct {
	ID       int64     `json:"id"`
	Source   string    `json:"source"`
	Session  string    `json:"session,omitempty"`
	Started  time.Time `json:"started"`
	Method   string    `json:"method"`
	URL      string    `json:"url"`
	Host     string    `json:"host"`
	Path     string    `json:"path"`
	Proto    string    `json:"proto,omitempty"` // HTTP/1.1, HTTP/2
	Kind     string    `json:"kind"`            // http, ws, sse, grpc, tunnel
	Type     string    `json:"type,omitempty"`  // json, html, image, …
	Status   int       `json:"status,omitempty"`
	State    string    `json:"state"` // pending, paused, done, error
	Error    string    `json:"error,omitempty"`
	ReqSize  int64     `json:"reqSize"`
	ResSize  int64     `json:"resSize"`
	Duration int64     `json:"duration,omitempty"` // ms
	Rule     string    `json:"rule,omitempty"`     // the rule that matched
	Mocked   bool      `json:"mocked,omitempty"`
	Pinned   bool      `json:"pinned,omitempty"`
	Messages int       `json:"messages,omitempty"` // WebSocket, SSE, gRPC
}

// Exchange states.
const (
	TrafficPending = "pending"
	TrafficHeld    = "paused"
	TrafficDone    = "done"
	TrafficFailed  = "error"
)

// Exchange kinds.
const (
	TrafficHTTP   = "http"
	TrafficWS     = "ws"
	TrafficSSE    = "sse"
	TrafficGRPC   = "grpc"
	TrafficTunnel = "tunnel"
)

// TrafficListParams lists the exchanges matching Filter (the filter
// language, docs/traffic.md), newest last, After an id, at most Limit.
type TrafficListParams struct {
	Filter string `json:"filter,omitempty"`
	After  int64  `json:"after,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// TrafficList is a page of exchanges.
type TrafficList struct {
	Exchanges []TrafficSummary `json:"exchanges"`
	More      bool             `json:"more,omitempty"`
}

// TrafficRef names an exchange; Reveal shows secrets unmasked.
type TrafficRef struct {
	ID     int64 `json:"id"`
	Reveal bool  `json:"reveal,omitempty"`
}

// Header is a header field, kept in order.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TrafficExchange is an exchange in full, but its bodies.
type TrafficExchange struct {
	TrafficSummary
	ReqHeaders []Header       `json:"reqHeaders"`
	ResHeaders []Header       `json:"resHeaders,omitempty"`
	Trailers   []Header       `json:"trailers,omitempty"`
	ReqType    string         `json:"reqType,omitempty"`
	ResType    string         `json:"resType,omitempty"`
	ReqKept    bool           `json:"reqKept"` // the body was kept, not only its size
	ResKept    bool           `json:"resKept"`
	Timings    TrafficTimings `json:"timings"`
	TLS        *TrafficTLS    `json:"tls,omitempty"`
	Remote     string         `json:"remote,omitempty"` // the upstream address
	Masked     bool           `json:"masked,omitempty"`
}

// TrafficTimings are milliseconds from the exchange's start.
type TrafficTimings struct {
	Connect   int64 `json:"connect,omitempty"`
	TLS       int64 `json:"tls,omitempty"`
	Sent      int64 `json:"sent,omitempty"`
	FirstByte int64 `json:"firstByte,omitempty"`
	Done      int64 `json:"done,omitempty"`
}

// TrafficTLS is the client's TLS with the proxy.
type TrafficTLS struct {
	Version    string `json:"version"`
	Cipher     string `json:"cipher"`
	ServerName string `json:"serverName,omitempty"`
	ALPN       string `json:"alpn,omitempty"`
}

// TrafficBodyParams reads a body: "request" or "response", decoded from
// its content encoding unless Raw, Size bytes from Offset.
type TrafficBodyParams struct {
	ID     int64  `json:"id"`
	Which  string `json:"which"`
	Raw    bool   `json:"raw,omitempty"`
	Offset int64  `json:"offset,omitempty"`
	Size   int    `json:"size,omitempty"`
	Reveal bool   `json:"reveal,omitempty"`
}

// TrafficBody is a part of a body.
type TrafficBody struct {
	Data        []byte `json:"data"`
	Total       int64  `json:"total"` // the whole body's size, decoded
	ContentType string `json:"contentType,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"` // past what is kept
}

// TrafficMessagesParams reads an exchange's messages (WebSocket frames,
// SSE events, gRPC messages), Limit from Offset.
type TrafficMessagesParams struct {
	ID     int64 `json:"id"`
	Offset int   `json:"offset,omitempty"`
	Limit  int   `json:"limit,omitempty"`
}

// TrafficMessage is one message of a stream.
type TrafficMessage struct {
	Sent   bool      `json:"sent"` // by the client
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"` // text, binary, ping, pong, close, event, message
	Data   []byte    `json:"data"`
	Size   int64     `json:"size"`
	Event  string    `json:"event,omitempty"` // SSE's event name
	Mocked bool      `json:"mocked,omitempty"`
}

// TrafficPinParams pins exchanges, kept when the rest is cleared.
type TrafficPinParams struct {
	IDs    []int64 `json:"ids"`
	Pinned bool    `json:"pinned"`
}

// TrafficClearParams clears the exchanges matching Filter ("" for all),
// pinned ones kept unless Pinned.
type TrafficClearParams struct {
	Filter string `json:"filter,omitempty"`
	Pinned bool   `json:"pinned,omitempty"`
}

// TrafficRulesParams names the rules of the project at Project ("" for the
// host's own).
type TrafficRulesParams struct {
	Project string `json:"project,omitempty"`
}

// TrafficRules are a project's rules, in order: the first that matches
// an exchange acts on it.
type TrafficRules struct {
	Project string        `json:"project,omitempty"`
	Rules   []TrafficRule `json:"rules"`
	// Filters are filters saved with the project, to pick again.
	Filters []string `json:"filters,omitempty"`
}

// TrafficRule matches exchanges and acts on them.
type TrafficRule struct {
	ID      string        `json:"id"`
	Name    string        `json:"name,omitempty"`
	Enabled bool          `json:"enabled"`
	Match   TrafficMatch  `json:"match"`
	Action  TrafficAction `json:"action"`
	Hits    int           `json:"hits,omitempty"`
	Problem string        `json:"problem,omitempty"` // why it can't apply
}

// TrafficMatch is what a rule matches; empty fields match anything.
type TrafficMatch struct {
	Methods []string `json:"methods,omitempty"`
	// Host matches a host: "example.com", "*.example.com" (subdomains),
	// ".example.com" (it and its subdomains).
	Host string `json:"host,omitempty"`
	Path string `json:"path,omitempty"`
	// PathMode is exact, prefix (the default), glob or regex.
	PathMode string            `json:"pathMode,omitempty"`
	Query    map[string]string `json:"query,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"` // value contained, case kept
	Body     string            `json:"body,omitempty"`
	BodyMode string            `json:"bodyMode,omitempty"` // contains (the default), regex
	JSON     map[string]string `json:"json,omitempty"`     // a.b.0.c → its value as text
	Source   string            `json:"source,omitempty"`
	Session  string            `json:"session,omitempty"`
	WS       bool              `json:"ws,omitempty"` // WebSocket upgrades only
}

// Rule actions.
const (
	ActionPass       = "pass"
	ActionMock       = "mock"
	ActionForward    = "forward"
	ActionTransform  = "transform"
	ActionFail       = "fail"
	ActionBreakpoint = "breakpoint"
)

// TrafficAction is what a rule does. Delay and Throttle apply with any
// kind.
type TrafficAction struct {
	Kind string `json:"kind"`
	// ActionMock: the answer, its body typed or the project's file.
	Status   int      `json:"status,omitempty"`
	Headers  []Header `json:"headers,omitempty"`
	Body     string   `json:"body,omitempty"`
	BodyFile string   `json:"bodyFile,omitempty"`
	Times    int      `json:"times,omitempty"` // 0: always
	// ActionForward: to this origin (scheme://host[:port]), the path kept.
	Target   string `json:"target,omitempty"`
	KeepHost bool   `json:"keepHost,omitempty"`
	// ActionTransform.
	Request  *TrafficTransform `json:"request,omitempty"`
	Response *TrafficTransform `json:"response,omitempty"`
	// ActionFail: close, reset, timeout, or status (a 5xx answer).
	Fail string `json:"fail,omitempty"`
	// ActionBreakpoint: request, response or both.
	Break string `json:"break,omitempty"`
	// WebSocket messages sent to the client once connected, when mocking
	// one; Echo sends its messages back.
	Messages []string `json:"messages,omitempty"`
	Echo     bool     `json:"echo,omitempty"`

	DelayMs  int `json:"delayMs,omitempty"`
	Throttle int `json:"throttle,omitempty"` // bytes per second
}

// TrafficTransform changes a request or a response on the way.
type TrafficTransform struct {
	SetHeaders    []Header          `json:"setHeaders,omitempty"`
	RemoveHeaders []string          `json:"removeHeaders,omitempty"`
	Replace       []TrafficReplace  `json:"replace,omitempty"`
	JSON          map[string]string `json:"json,omitempty"` // a.b → a JSON value; "" deletes
	Body          *string           `json:"body,omitempty"` // the whole body
	Status        int               `json:"status,omitempty"`
	URL           string            `json:"url,omitempty"` // a request's
}

// TrafficReplace replaces text in a body: From as text, or as a regular
// expression when Regex.
type TrafficReplace struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Regex bool   `json:"regex,omitempty"`
}

// TrafficPaused is an exchange waiting at a breakpoint.
type TrafficPaused struct {
	ID      int64     `json:"id"`
	Stage   string    `json:"stage"` // request or response
	Source  string    `json:"source"`
	Session string    `json:"session,omitempty"`
	Method  string    `json:"method"`
	URL     string    `json:"url"`
	Status  int       `json:"status,omitempty"`
	Headers []Header  `json:"headers"`
	Body    []byte    `json:"body,omitempty"`
	Since   time.Time `json:"since"`
}

// TrafficResumeParams lets a paused exchange go on, with Edit's changes,
// or drops it.
type TrafficResumeParams struct {
	ID   int64        `json:"id"`
	Drop bool         `json:"drop,omitempty"`
	Edit *TrafficEdit `json:"edit,omitempty"`
}

// TrafficEdit changes a paused request or response; nil fields stay.
type TrafficEdit struct {
	Method  string   `json:"method,omitempty"`
	URL     string   `json:"url,omitempty"`
	Status  int      `json:"status,omitempty"`
	Headers []Header `json:"headers,omitempty"`
	Body    *[]byte  `json:"body,omitempty"`
}

// TrafficSendParams sends a request from the host, through the rules
// unless Direct, recorded as an exchange of Source ("compose" by
// default).
type TrafficSendParams struct {
	Method  string   `json:"method"`
	URL     string   `json:"url"`
	Headers []Header `json:"headers,omitempty"`
	Body    []byte   `json:"body,omitempty"`
	Direct  bool     `json:"direct,omitempty"`
	Project string   `json:"project,omitempty"` // whose rules
	Session string   `json:"session,omitempty"`
}

// TrafficExportParams exports the exchanges matching Filter (or IDs) as
// HAR, secrets masked unless Reveal.
type TrafficExportParams struct {
	Filter string  `json:"filter,omitempty"`
	IDs    []int64 `json:"ids,omitempty"`
	Reveal bool    `json:"reveal,omitempty"`
}

// TrafficImportParams imports a HAR written with transfer.write as ID.
type TrafficImportParams struct {
	ID string `json:"id"`
}

// TrafficSettings are a host's interceptor's settings.
type TrafficSettings struct {
	// Insecure are hosts whose certificates aren't checked upstream
	// ("*.test" patterns as rules take); loopback is always among them,
	// as development servers sign their own.
	Insecure []string `json:"insecure,omitempty"`
	// Upstream is a proxy the interceptor sends through ("" for none).
	Upstream string `json:"upstream,omitempty"`
	// MaxExchanges and MaxMB bound the capture (0: the defaults).
	MaxExchanges int `json:"maxExchanges,omitempty"`
	MaxMB        int `json:"maxMB,omitempty"`
	// NotifyErrors tells the app to say when an app's request fails or
	// is answered 5xx.
	NotifyErrors bool `json:"notifyErrors,omitempty"`
	// Hosts point names at other addresses (an IP, or host:port), as a
	// hosts file would, for the interceptor's connections.
	Hosts map[string]string `json:"hosts,omitempty"`
	// ClientCerts are certificates shown to hosts that ask for one.
	ClientCerts []TrafficClientCert `json:"clientCerts,omitempty"`
}

// TrafficClientCert is a client certificate for hosts matching Host
// (patterns as rules take), its PEM files on the interceptor's host.
type TrafficClientCert struct {
	Host string `json:"host"`
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

// TrafficReverseParams names a session's port to put a reverse proxy in
// front of.
type TrafficReverseParams struct {
	Session string `json:"session"`
	Port    int    `json:"port"`
}

// TrafficReverse is the reverse proxy's port, on the host's loopback.
type TrafficReverse struct {
	Port   int    `json:"port"`
	Source string `json:"source"`
}

// TrafficMCP says which agents got enowX's MCP server.
type TrafficMCP struct {
	Added []string `json:"added"`
	Notes []string `json:"notes,omitempty"`
}
