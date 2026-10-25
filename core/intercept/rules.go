package intercept

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/enowx/enowx/proto"
)

// Rules are compiled rules of one project, safe for use by several
// goroutines: hits and mocks' remaining times change as they apply.
type Rules struct {
	mu    sync.Mutex
	list  []*rule
	byID  map[string]*rule
	files func(name string) ([]byte, error) // the project's files, for mocks' bodies
}

type rule struct {
	proto.TrafficRule
	path  *regexp.Regexp // glob and regex paths
	body  *regexp.Regexp
	repls map[*proto.TrafficReplace]*regexp.Regexp
	used  int // times a limited mock answered
}

// CompileRules compiles rules, marking those that can't apply with their
// problem; files reads a mock's body file, relative to the project.
func CompileRules(in []proto.TrafficRule, files func(string) ([]byte, error)) *Rules {
	rs := &Rules{byID: map[string]*rule{}, files: files}
	for _, r := range in {
		c := &rule{TrafficRule: r, repls: map[*proto.TrafficReplace]*regexp.Regexp{}}
		c.Problem = c.compile()
		rs.list = append(rs.list, c)
		rs.byID[r.ID] = c
	}
	return rs
}

func (r *rule) compile() string {
	m := r.Match
	switch m.PathMode {
	case "", "prefix", "exact":
	case "glob":
		re, err := regexp.Compile("^" + globToRegexp(m.Path) + "$")
		if err != nil {
			return "path: " + err.Error()
		}
		r.path = re
	case "regex":
		re, err := regexp.Compile(m.Path)
		if err != nil {
			return "path: " + err.Error()
		}
		r.path = re
	default:
		return "path mode " + strconv.Quote(m.PathMode) + " is unknown"
	}
	if m.BodyMode == "regex" && m.Body != "" {
		re, err := regexp.Compile(m.Body)
		if err != nil {
			return "body: " + err.Error()
		}
		r.body = re
	}
	a := r.Action
	switch a.Kind {
	case proto.ActionPass, "":
	case proto.ActionMock:
		if a.Status != 0 && (a.Status < 100 || a.Status > 999) {
			return "status " + strconv.Itoa(a.Status) + " isn't an HTTP status"
		}
	case proto.ActionForward:
		u, err := url.Parse(a.Target)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return "forward needs an origin such as https://staging.example.com"
		}
	case proto.ActionTransform:
		for _, t := range []*proto.TrafficTransform{a.Request, a.Response} {
			if t == nil {
				continue
			}
			for i := range t.Replace {
				rp := &t.Replace[i]
				if rp.Regex {
					re, err := regexp.Compile(rp.From)
					if err != nil {
						return "replace: " + err.Error()
					}
					r.repls[rp] = re
				}
			}
		}
	case proto.ActionFail:
		switch a.Fail {
		case "close", "reset", "timeout", "status", "":
		default:
			return "fail " + strconv.Quote(a.Fail) + " is unknown"
		}
	case proto.ActionBreakpoint:
		switch a.Break {
		case "request", "response", "both", "":
		default:
			return "breakpoint " + strconv.Quote(a.Break) + " is unknown"
		}
	default:
		return "action " + strconv.Quote(a.Kind) + " is unknown"
	}
	return ""
}

// globToRegexp makes a path glob a regular expression: * within a
// segment, ** across segments.
func globToRegexp(g string) string {
	var b strings.Builder
	for i := 0; i < len(g); i++ {
		switch c := g[i]; c {
		case '*':
			if i+1 < len(g) && g[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String()
}

// List is the rules with their hits now.
func (rs *Rules) List() []proto.TrafficRule {
	if rs == nil {
		return nil
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	out := make([]proto.TrafficRule, len(rs.list))
	for i, r := range rs.list {
		out[i] = r.TrafficRule
	}
	return out
}

// Hits are the rules' hits by id, to carry over when they are replaced.
func (rs *Rules) Hits() map[string]int {
	out := map[string]int{}
	if rs == nil {
		return out
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, r := range rs.list {
		out[r.ID] = r.Hits
	}
	return out
}

// request is what rules match a request on.
type request struct {
	method  string
	url     *url.URL
	header  http.Header
	body    []byte // nil when it wasn't read
	source  string
	session string
	ws      bool
}

// needsBody reports whether a rule could need the request's body to
// match or act: then the proxy reads it first.
func (rs *Rules) needsBody() bool {
	if rs == nil {
		return false
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, r := range rs.list {
		if !r.Enabled || r.Problem != "" {
			continue
		}
		if r.Match.Body != "" || len(r.Match.JSON) > 0 {
			return true
		}
		a := r.Action
		if a.Kind == proto.ActionTransform && a.Request != nil || a.Kind == proto.ActionBreakpoint && a.Break != "response" {
			return true
		}
	}
	return false
}

// match returns the first enabled rule matching req, counting its hit; a
// mock used up its times no longer matches.
func (rs *Rules) match(req request) (proto.TrafficRule, bool) {
	if rs == nil {
		return proto.TrafficRule{}, false
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, r := range rs.list {
		if !r.Enabled || r.Problem != "" || !r.matches(req) {
			continue
		}
		if r.Action.Kind == proto.ActionMock && r.Action.Times > 0 {
			if r.used >= r.Action.Times {
				continue
			}
			r.used++
		}
		r.Hits++
		return r.TrafficRule, true
	}
	return proto.TrafficRule{}, false
}

func (r *rule) matches(req request) bool {
	m := r.Match
	if len(m.Methods) > 0 && !containsFold(m.Methods, req.method) {
		return false
	}
	if m.WS && !req.ws {
		return false
	}
	if m.Source != "" && m.Source != req.source {
		return false
	}
	if m.Session != "" && m.Session != req.session {
		return false
	}
	if m.Host != "" && !HostMatches(m.Host, req.url.Hostname()) {
		return false
	}
	if m.Path != "" {
		p := req.url.Path
		switch m.PathMode {
		case "exact":
			if p != m.Path {
				return false
			}
		case "glob", "regex":
			if r.path == nil || !r.path.MatchString(p) {
				return false
			}
		default:
			if !strings.HasPrefix(p, m.Path) {
				return false
			}
		}
	}
	if len(m.Query) > 0 {
		q := req.url.Query()
		for k, v := range m.Query {
			if !q.Has(k) || (v != "" && q.Get(k) != v) {
				return false
			}
		}
	}
	for k, v := range m.Headers {
		got := req.header.Values(k)
		if len(got) == 0 || (v != "" && !strings.Contains(strings.Join(got, ", "), v)) {
			return false
		}
	}
	if m.Body != "" {
		if req.body == nil {
			return false
		}
		if r.body != nil {
			if !r.body.Match(req.body) {
				return false
			}
		} else if !bytes.Contains(req.body, []byte(m.Body)) {
			return false
		}
	}
	if len(m.JSON) > 0 {
		var v any
		if req.body == nil || json.Unmarshal(req.body, &v) != nil {
			return false
		}
		for p, want := range m.JSON {
			got, ok := jsonAt(v, p)
			if !ok || (want != "" && jsonText(got) != want) {
				return false
			}
		}
	}
	return true
}

// HostMatches reports whether host matches pattern: "example.com" that
// host, "*.example.com" its subdomains, ".example.com" it and its
// subdomains.
func HostMatches(pattern, host string) bool {
	pattern, host = strings.ToLower(pattern), strings.ToLower(strings.TrimSuffix(host, "."))
	switch {
	case strings.HasPrefix(pattern, "*."):
		return strings.HasSuffix(host, pattern[1:])
	case strings.HasPrefix(pattern, "."):
		return host == pattern[1:] || strings.HasSuffix(host, pattern)
	}
	return host == pattern
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// jsonAt is v's value at a dotted path ("a.b.0.c").
func jsonAt(v any, p string) (any, bool) {
	for _, part := range strings.Split(p, ".") {
		switch t := v.(type) {
		case map[string]any:
			x, ok := t[part]
			if !ok {
				return nil, false
			}
			v = x
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(t) {
				return nil, false
			}
			v = t[i]
		default:
			return nil, false
		}
	}
	return v, true
}

func jsonText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// setJSON sets v's value at a dotted path to value (JSON, or text taken
// as a string), deleting it when value is "".
func setJSON(v any, p, value string) any {
	parts := strings.Split(p, ".")
	var parse func(cur any, i int) any
	parse = func(cur any, i int) any {
		key := parts[i]
		last := i == len(parts)-1
		switch t := cur.(type) {
		case map[string]any:
			if last {
				if value == "" {
					delete(t, key)
				} else {
					t[key] = jsonValue(value)
				}
				return t
			}
			next, ok := t[key]
			if !ok {
				next = map[string]any{}
			}
			t[key] = parse(next, i+1)
			return t
		case []any:
			n, err := strconv.Atoi(key)
			if err != nil || n < 0 || n >= len(t) {
				return t
			}
			if last {
				if value == "" {
					return append(t[:n], t[n+1:]...)
				}
				t[n] = jsonValue(value)
				return t
			}
			t[n] = parse(t[n], i+1)
			return t
		}
		return cur
	}
	return parse(v, 0)
}

func jsonValue(s string) any {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		return v
	}
	return s
}

// transformHeaders sets and removes headers as t says.
func transformHeaders(h http.Header, t *proto.TrafficTransform) {
	for _, n := range t.RemoveHeaders {
		h.Del(n)
	}
	for _, x := range t.SetHeaders {
		h.Set(x.Name, x.Value)
	}
}

// transformBody changes a decoded body as t says.
func (rs *Rules) transformBody(ruleID string, data []byte, t *proto.TrafficTransform) []byte {
	if t.Body != nil {
		return []byte(*t.Body)
	}
	rs.mu.Lock()
	r := rs.byID[ruleID]
	rs.mu.Unlock()
	for i := range t.Replace {
		rp := &t.Replace[i]
		if re := r.repls[rp]; re != nil {
			data = re.ReplaceAll(data, []byte(rp.To))
		} else if rp.From != "" {
			data = bytes.ReplaceAll(data, []byte(rp.From), []byte(rp.To))
		}
	}
	if len(t.JSON) > 0 {
		var v any
		if json.Unmarshal(data, &v) == nil {
			for p, val := range t.JSON {
				v = setJSON(v, p, val)
			}
			if out, err := json.Marshal(v); err == nil {
				data = out
			}
		}
	}
	return data
}

// mockBody is a mock's body: typed, or read from the project's file.
func (rs *Rules) mockBody(a proto.TrafficAction) ([]byte, error) {
	if a.BodyFile == "" {
		return []byte(a.Body), nil
	}
	if rs.files == nil {
		return nil, fmt.Errorf("the body's file %s can't be read here", a.BodyFile)
	}
	clean := path.Clean("/" + a.BodyFile)[1:]
	return rs.files(clean)
}
