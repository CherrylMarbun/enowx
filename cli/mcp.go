package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/version"
)

// enowx-app mcp serves enowX's tools to agents over MCP (JSON-RPC on
// stdin and stdout, a message a line): the traffic the host caught, the
// session's Browser, the host's Android devices, and other sessions. Each tool runs the command line's command of the
// same name, so both say the same.

// mcpTool is a tool: its schema, and the command line it runs.
type mcpTool struct {
	name, description string
	props             map[string]mcpProp
	required          []string
	args              func(a map[string]any) []string
}

type mcpProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

func str(a map[string]any, k string) string {
	switch v := a[k].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

// withFlag adds --name value when the argument is given.
func withFlag(args []string, a map[string]any, k, name string) []string {
	if v := str(a, k); v != "" && v != "false" {
		if v == "true" {
			return append(args, "--"+name)
		}
		return append(args, "--"+name, v)
	}
	return args
}

var filterProp = mcpProp{"string", "A filter: domain:example.com (and subdomains), host:*.example.com, path:/api/*, method:POST, status:4xx, status:>=500, type:json|ws|grpc|sse, header:name=value, body:\"text\", size:>1mb, time:>500ms, since:10m, is:mocked|error|paused; -x negates."}

var mcpTools = []mcpTool{
	{name: "traffic_list", description: "List the HTTP exchanges enowX caught for this session (all of the host's with all=true), newest last: number, status, method, URL, type, size, time.",
		props: map[string]mcpProp{"filter": filterProp, "all": {"boolean", "Every source's traffic, not only this session's."}, "limit": {"number", "How many, newest (default 100)."}},
		args: func(a map[string]any) []string {
			return withFlag(withFlag(withFlag([]string{"list"}, a, "filter", "filter"), a, "all", "all"), a, "limit", "limit")
		}},
	{name: "traffic_show", description: "Show one exchange: request and response headers and bodies (decoded, secrets masked), and WebSocket, SSE or gRPC messages.",
		props:    map[string]mcpProp{"id": {"number", "The exchange's number, from traffic_list."}, "body": {"string", "Only this body, whole: request or response."}},
		required: []string{"id"},
		args:     func(a map[string]any) []string { return withFlag([]string{"show", str(a, "id")}, a, "body", "body") }},
	{name: "traffic_watch", description: "Wait for the next exchange matching a filter (or any), as an app makes it, and show it.",
		props: map[string]mcpProp{"filter": filterProp, "timeout_seconds": {"number", "How long to wait (default 60)."}, "all": {"boolean", "Every source's traffic."}},
		args: func(a map[string]any) []string {
			t := str(a, "timeout_seconds")
			if t == "" {
				t = "60"
			}
			return withFlag(withFlag([]string{"watch", "--until-match", "--timeout", t + "s"}, a, "filter", "filter"), a, "all", "all")
		}},
	{name: "traffic_mock", description: "Answer matching requests from a mock, to test an error path or an API not built yet. Saved in the project's rules (.enowx/traffic.json).",
		props: map[string]mcpProp{"method": {"string", "GET, POST… (comma separated)"}, "host": {"string", "api.example.com, *.example.com"}, "path": {"string", "A path prefix, or a glob with *"},
			"status": {"number", "The status to answer (default 200)."}, "body": {"string", "The body to answer."}, "header": {"string", "A header to answer, Name: value."},
			"times": {"number", "Answer only this many times."}, "delay_ms": {"number", "Wait this long first."}},
		args: func(a map[string]any) []string {
			args := []string{"mock"}
			for _, k := range [][2]string{{"method", "method"}, {"host", "host"}, {"path", "path"}, {"status", "status"}, {"body", "body"}, {"header", "header"}, {"times", "times"}, {"delay_ms", "delay"}} {
				args = withFlag(args, a, k[0], k[1])
			}
			return args
		}},
	{name: "traffic_breakpoint", description: "Pause matching requests (or their responses) until the user resumes them in enowX.",
		props: map[string]mcpProp{"method": {"string", "GET, POST…"}, "host": {"string", "A host"}, "path": {"string", "A path prefix or glob"}, "stage": {"string", "request, response or both"}},
		args: func(a map[string]any) []string {
			return withFlag(withFlag(withFlag(withFlag([]string{"breakpoint"}, a, "method", "method"), a, "host", "host"), a, "path", "path"), a, "stage", "stage")
		}},
	{name: "traffic_rules", description: "List the project's traffic rules: mocks, forwards, breakpoints, with their ids and hits.",
		args: func(map[string]any) []string { return []string{"rules"} }},
	{name: "traffic_rule_remove", description: "Remove a traffic rule by its id.",
		props: map[string]mcpProp{"id": {"string", "The rule's id, from traffic_rules."}}, required: []string{"id"},
		args: func(a map[string]any) []string { return []string{"rules", "rm", str(a, "id")} }},
	{name: "traffic_send", description: "Send an HTTP request from the host, through the rules unless direct, and show the exchange.",
		props: map[string]mcpProp{"method": {"string", "GET by default, POST with a body"}, "url": {"string", "https://…"}, "header": {"string", "A header, Name: value"},
			"body": {"string", "The request's body"}, "direct": {"boolean", "Around the rules"}},
		required: []string{"url"},
		args: func(a map[string]any) []string {
			args := []string{"send"}
			if m := str(a, "method"); m != "" {
				args = append(args, "-X", m)
			}
			if h := str(a, "header"); h != "" {
				args = append(args, "-H", h)
			}
			if b := str(a, "body"); b != "" {
				args = append(args, "-d", b)
			}
			args = withFlag(args, a, "direct", "direct")
			return append(args, str(a, "url"))
		}},
	{name: "traffic_export", description: "Export exchanges as a HAR file, secrets masked.",
		props: map[string]mcpProp{"filter": filterProp, "path": {"string", "Where to write it"}}, required: []string{"path"},
		args: func(a map[string]any) []string {
			return withFlag(withFlag([]string{"export"}, a, "filter", "filter"), a, "path", "out")
		}},
	{name: "traffic_attach", description: "Catch more of this session's traffic: terminal (a new tab), session (its new tabs), docker, browser (with program chrome, chromium, edge, brave or firefox), manual.",
		props:    map[string]mcpProp{"kind": {"string", "terminal, session, docker, browser, manual"}, "program": {"string", "For a browser: chrome, chromium, edge, brave or firefox"}},
		required: []string{"kind"},
		args: func(a map[string]any) []string {
			args := []string{"attach", str(a, "kind")}
			if p := str(a, "program"); p != "" {
				args = append(args, p)
			}
			return args
		}},
	{name: "traffic_clear", description: "Clear caught exchanges (matching a filter, or all).",
		props: map[string]mcpProp{"filter": filterProp},
		args:  func(a map[string]any) []string { return withFlag([]string{"clear"}, a, "filter", "filter") }},
}

// browserTools drive the session's Browser in the app.
var browserTools = []mcpTool{
	{name: "browser_open", description: "Open a page in this session's Browser in enowX: a URL, a host, or a path of its dev server.",
		props: map[string]mcpProp{"url": {"string", "What to open"}}, required: []string{"url"},
		args: func(a map[string]any) []string { return []string{"open", str(a, "url")} }},
	{name: "browser_snapshot", description: "The Browser's page as text, its controls numbered for browser_click and browser_fill.",
		args: func(map[string]any) []string { return []string{"snapshot"} }},
	{name: "browser_click", description: "Click a control of the page: its number from browser_snapshot, or a CSS selector.",
		props: map[string]mcpProp{"ref": {"string", "The control"}}, required: []string{"ref"},
		args: func(a map[string]any) []string { return []string{"click", str(a, "ref")} }},
	{name: "browser_fill", description: "Type text into a field of the page.",
		props: map[string]mcpProp{"ref": {"string", "The field"}, "text": {"string", "What to type"}}, required: []string{"ref", "text"},
		args: func(a map[string]any) []string { return []string{"fill", str(a, "ref"), str(a, "text")} }},
	{name: "browser_screenshot", description: "Save a PNG of the page.",
		props: map[string]mcpProp{"path": {"string", "Where to write it"}}, required: []string{"path"},
		args: func(a map[string]any) []string { return []string{"screenshot", str(a, "path")} }},
}

// androidTools drive the host's Android devices.
var androidTools = []mcpTool{
	{name: "android_status", description: "The host's Android SDK, virtual phones (AVDs) and the devices adb sees, with their serials.",
		args: func(map[string]any) []string { return []string{"status"} }},
	{name: "android_start", description: "Start a virtual phone by its AVD name, from android_status.",
		props: map[string]mcpProp{"avd": {"string", "The AVD's name"}}, required: []string{"avd"},
		args: func(a map[string]any) []string { return []string{"start", str(a, "avd")} }},
	{name: "android_run", description: "Run this session's app (Flutter, Expo, React Native, Capacitor, NativePHP, Gradle) on a device, in a new tab of the session; prints the tab, to read with session_read.",
		props:    map[string]mcpProp{"serial": {"string", "The device's serial, from android_status"}, "command": {"string", "The command to run instead of the project's own"}},
		required: []string{"serial"},
		args: func(a map[string]any) []string {
			return withFlag([]string{"run", str(a, "serial")}, a, "command", "command")
		}},
	{name: "android_logcat", description: "A device's log so far, only an app's with package, at a level and above.",
		props:    map[string]mcpProp{"serial": {"string", "The device's serial"}, "package": {"string", "The app's package, com.example.app"}, "level": {"string", "V, D, I, W or E"}},
		required: []string{"serial"},
		args: func(a map[string]any) []string {
			return withFlag(withFlag([]string{"logcat", str(a, "serial")}, a, "package", "package"), a, "level", "level")
		}},
}

// sessionTools start other agents and follow them, and say where this
// session is at.
var sessionTools = []mcpTool{
	{name: "sessions_list", description: "The sessions on this host: ID, agent, state, project, branch, name.",
		args: func(map[string]any) []string { return []string{"ls"} }},
	{name: "session_spawn", description: "Start another coding agent in a session of its own, which the user sees in enowX; prints its ID.",
		props: map[string]mcpProp{"prompt": {"string", "Its task"}, "agent": {"string", "claude, codex, gemini, opencode… (default claude)"},
			"branch": {"string", "A new branch in a worktree of its own, so it doesn't touch this session's files"}, "yolo": {"boolean", "Skip its permission prompts"}},
		required: []string{"prompt"},
		args: func(a map[string]any) []string {
			return append(withFlag(withFlag(withFlag([]string{"spawn"}, a, "agent", "agent"), a, "branch", "branch"), a, "yolo", "yolo"), str(a, "prompt"))
		}},
	{name: "session_wait", description: "Wait until a session's agent is done or asks something.",
		props: map[string]mcpProp{"id": {"string", "The session"}, "timeout": {"string", "How long at most, as 10m (default 30m)"}}, required: []string{"id"},
		args: func(a map[string]any) []string {
			return withFlag([]string{"wait", str(a, "id")}, a, "timeout", "timeout")
		}},
	{name: "session_read", description: "What a session's or a tab's screen shows.",
		props: map[string]mcpProp{"id": {"string", "The session or tab"}}, required: []string{"id"},
		args: func(a map[string]any) []string { return []string{"read", str(a, "id")} }},
	{name: "session_send", description: "Type a message into a session and send it.",
		props: map[string]mcpProp{"id": {"string", "The session"}, "text": {"string", "The message"}}, required: []string{"id", "text"},
		args: func(a map[string]any) []string { return []string{"send", str(a, "id"), str(a, "text")} }},
	{name: "session_note", description: "Say where this session is at, in a line shown under it in enowX (at milestones).",
		props: map[string]mcpProp{"text": {"string", "Where you are"}}, required: []string{"text"},
		args: func(a map[string]any) []string { return []string{"note", "self", str(a, "text")} }},
}

// mcpGroup is tools with the command line's command each runs under.
type mcpGroup struct {
	tools []mcpTool
	run   func(r runner, args []string) error
}

// mcpGroups are every tool; a function, as the commands they run serve
// mcp too.
func mcpGroups() []mcpGroup {
	return []mcpGroup{
		{mcpTools, runner.traffic},
		{browserTools, runner.browser},
		{androidTools, runner.android},
		{sessionTools, runner.command},
	}
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcp serves the tools on in and out until in ends.
func (r runner) mcp(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(out)
	reply := func(id json.RawMessage, result any, rpcErr any) {
		msg := map[string]any{"jsonrpc": "2.0", "id": id}
		if rpcErr != nil {
			msg["error"] = rpcErr
		} else {
			msg["result"] = result
		}
		enc.Encode(msg)
	}
	tools := map[string]mcpTool{}
	run := map[string]func(runner, []string) error{}
	for _, g := range mcpGroups() {
		for _, t := range g.tools {
			tools[t.name], run[t.name] = t, g.run
		}
	}
	for sc.Scan() {
		var m rpcMessage
		if json.Unmarshal(sc.Bytes(), &m) != nil || m.ID == nil {
			continue // notifications need no answer
		}
		switch m.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			json.Unmarshal(m.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2025-06-18"
			}
			reply(m.ID, map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "enowx", "version": version.String},
				"instructions":    agent.Guide,
			}, nil)
		case "ping":
			reply(m.ID, map[string]any{}, nil)
		case "tools/list":
			var list []any
			for _, g := range mcpGroups() {
				for _, t := range g.tools {
					props := t.props
					if props == nil {
						props = map[string]mcpProp{}
					}
					schema := map[string]any{"type": "object", "properties": props}
					if len(t.required) > 0 {
						schema["required"] = t.required
					}
					list = append(list, map[string]any{"name": t.name, "description": t.description, "inputSchema": schema})
				}
			}
			reply(m.ID, map[string]any{"tools": list}, nil)
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if err := json.Unmarshal(m.Params, &p); err != nil {
				reply(m.ID, nil, map[string]any{"code": -32602, "message": err.Error()})
				continue
			}
			t, ok := tools[p.Name]
			if !ok {
				reply(m.ID, nil, map[string]any{"code": -32602, "message": "no tool " + p.Name})
				continue
			}
			reply(m.ID, r.callTool(t, run[t.name], p.Arguments), nil)
		default:
			reply(m.ID, nil, map[string]any{"code": -32601, "message": fmt.Sprintf("no method %s", m.Method)})
		}
	}
	return sc.Err()
}

// callTool runs a tool's command line and says what it printed.
func (r runner) callTool(t mcpTool, run func(runner, []string) error, a map[string]any) map[string]any {
	result := func(text string, failed bool) map[string]any {
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}, "isError": failed}
	}
	for _, k := range t.required {
		if str(a, k) == "" {
			return result(k+" is required", true)
		}
	}
	var buf bytes.Buffer
	rr := runner{c: r.c, out: &buf}
	err := run(rr, t.args(a))
	text := buf.String()
	if err != nil {
		text = strings.TrimSpace(text + "\n" + err.Error())
	}
	if text == "" {
		text = "Done."
	}
	return result(text, err != nil)
}
