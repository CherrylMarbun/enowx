package intercept

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/enowx/enowx/proto"
)

// Filter is a parsed filter: conditions that all must hold. The same
// language serves the app's filter bar, the command line and agents'
// tools (docs/traffic.md):
//
//	domain:example.com   the host and its subdomains
//	host:*.example.com   subdomains only; host:api.example.com that host
//	path:/api/*  url:<regexp>  query:page=2
//	method:POST  status:4xx  status:>=500  status:200-299
//	type:json|html|image|ws|grpc|sse|graphql|tunnel
//	header:content-type=application/json  header:x-trace
//	body:"invalid token"  graphql:op=GetUser
//	size:>1mb  time:>500ms  since:10m
//	source:<id>  session:<id>  is:paused|error|pinned|mocked|pending
//
// A condition starting with "-" is negated; other words match the URL,
// then headers and bodies.
type Filter struct {
	conds []cond
}

type cond struct {
	neg bool
	fn  func(c *checked) bool
}

// checked is an exchange being matched, its details and bodies read only
// when a condition needs them.
type checked struct {
	ex      *Exchange
	sum     proto.TrafficSummary
	details *proto.TrafficExchange
	bodies  []byte
	now     time.Time
}

func (c *checked) detail() *proto.TrafficExchange {
	if c.details == nil {
		d := c.ex.Details(true)
		c.details = &d
	}
	return c.details
}

func (c *checked) body() []byte {
	if c.bodies == nil {
		var all []byte
		for _, which := range []string{"request", "response"} {
			b := c.ex.rawBody(which)
			dec, _ := decode(b.data, b.encoding)
			all = append(append(all, dec...), '\n')
		}
		c.bodies = all
	}
	return c.bodies
}

// ParseFilter parses a filter; an empty one matches everything.
func ParseFilter(s string) (*Filter, error) {
	f := &Filter{}
	for _, tok := range splitFilter(s) {
		neg := false
		if strings.HasPrefix(tok, "-") && len(tok) > 1 {
			neg, tok = true, tok[1:]
		}
		key, val, hasKey := strings.Cut(tok, ":")
		if !hasKey || !isFilterKey(key) {
			text := strings.ToLower(tok)
			f.conds = append(f.conds, cond{neg: neg, fn: func(c *checked) bool { return freeText(c, text) }})
			continue
		}
		val = unquote(val)
		fn, err := filterCond(strings.ToLower(key), val)
		if err != nil {
			return nil, fmt.Errorf("filter %s: %w", tok, err)
		}
		f.conds = append(f.conds, cond{neg: neg, fn: fn})
	}
	return f, nil
}

var filterKeys = []string{"domain", "host", "path", "url", "query", "method", "status", "type", "header", "body", "graphql", "size", "time", "since", "source", "session", "is", "rule"}

func isFilterKey(k string) bool {
	for _, x := range filterKeys {
		if strings.EqualFold(x, k) {
			return true
		}
	}
	return false
}

// Match reports whether ex matches every condition.
func (f *Filter) Match(ex *Exchange) bool {
	if f == nil || len(f.conds) == 0 {
		return true
	}
	c := &checked{ex: ex, sum: ex.Summary(), now: time.Now()}
	for _, cd := range f.conds {
		if cd.fn(c) == cd.neg {
			return false
		}
	}
	return true
}

func filterCond(key, val string) (func(*checked) bool, error) {
	low := strings.ToLower(val)
	switch key {
	case "domain":
		return func(c *checked) bool { return HostMatches("."+strings.TrimPrefix(low, "."), c.sum.Host) }, nil
	case "host":
		return func(c *checked) bool { return HostMatches(low, c.sum.Host) }, nil
	case "path":
		re, err := regexp.Compile("^" + globToRegexp(val) + `(\?.*)?$`)
		if err != nil {
			return nil, err
		}
		prefix := !strings.ContainsAny(val, "*?")
		return func(c *checked) bool {
			p := c.sum.Path
			if prefix {
				return strings.HasPrefix(p, val)
			}
			return re.MatchString(p)
		}, nil
	case "url":
		re, err := regexp.Compile(val)
		if err != nil {
			return nil, err
		}
		return func(c *checked) bool { return re.MatchString(c.sum.URL) }, nil
	case "query":
		k, v, _ := strings.Cut(val, "=")
		return func(c *checked) bool {
			_, q, _ := strings.Cut(c.sum.Path, "?")
			for _, part := range strings.Split(q, "&") {
				pk, pv, _ := strings.Cut(part, "=")
				if pk == k && (v == "" || pv == v) {
					return true
				}
			}
			return false
		}, nil
	case "method":
		methods := strings.Split(strings.ToUpper(val), "|")
		return func(c *checked) bool { return containsFold(methods, c.sum.Method) }, nil
	case "status":
		return statusCond(low)
	case "type":
		types := strings.Split(low, "|")
		return func(c *checked) bool {
			for _, t := range types {
				if t == c.sum.Type || t == c.sum.Kind || (t == "graphql" && isGraphQL(c)) {
					return true
				}
			}
			return false
		}, nil
	case "header":
		name, want, _ := strings.Cut(val, "=")
		want = strings.ToLower(want)
		return func(c *checked) bool {
			d := c.detail()
			for _, hs := range [][]proto.Header{d.ReqHeaders, d.ResHeaders} {
				for _, h := range hs {
					if strings.EqualFold(h.Name, name) && (want == "" || strings.Contains(strings.ToLower(h.Value), want)) {
						return true
					}
				}
			}
			return false
		}, nil
	case "body":
		return func(c *checked) bool { return bytes.Contains(bytes.ToLower(c.body()), []byte(low)) }, nil
	case "graphql":
		op := strings.TrimPrefix(val, "op=")
		return func(c *checked) bool { return isGraphQL(c) && (op == "" || graphQLOp(c) == op) }, nil
	case "size":
		return compareCond(val, parseSize, func(c *checked) int64 { return c.sum.ResSize + c.sum.ReqSize })
	case "time":
		return compareCond(val, parseMillis, func(c *checked) int64 { return c.sum.Duration })
	case "since":
		d, err := time.ParseDuration(val)
		if err != nil {
			return nil, err
		}
		return func(c *checked) bool { return c.sum.Started.After(c.now.Add(-d)) }, nil
	case "source":
		return func(c *checked) bool { return c.sum.Source == val || strings.HasPrefix(c.sum.Source, val+"-") }, nil
	case "session":
		return func(c *checked) bool { return c.sum.Session == val }, nil
	case "is", "rule":
		return func(c *checked) bool {
			switch low {
			case "paused":
				return c.sum.State == proto.TrafficHeld
			case "error", "failed":
				return c.sum.State == proto.TrafficFailed
			case "pending":
				return c.sum.State == proto.TrafficPending
			case "pinned":
				return c.sum.Pinned
			case "mocked":
				return c.sum.Mocked
			case "matched", "any":
				return c.sum.Rule != ""
			}
			return c.sum.Rule == val
		}, nil
	}
	return nil, fmt.Errorf("unknown key")
}

func statusCond(v string) (func(*checked) bool, error) {
	if len(v) == 3 && strings.HasSuffix(v, "xx") && v[0] >= '1' && v[0] <= '5' {
		class := int(v[0] - '0')
		return func(c *checked) bool { return c.sum.Status/100 == class }, nil
	}
	if lo, hi, ok := strings.Cut(v, "-"); ok {
		a, err1 := strconv.Atoi(lo)
		b, err2 := strconv.Atoi(hi)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("a status range is like 200-299")
		}
		return func(c *checked) bool { return c.sum.Status >= a && c.sum.Status <= b }, nil
	}
	return compareCond(v, func(s string) (int64, error) { n, err := strconv.Atoi(s); return int64(n), err }, func(c *checked) int64 { return int64(c.sum.Status) })
}

// compareCond is a comparison like ">1mb", "<=500ms" or "404".
func compareCond(v string, parse func(string) (int64, error), get func(*checked) int64) (func(*checked) bool, error) {
	op := ""
	for _, o := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(v, o) {
			op, v = o, v[len(o):]
			break
		}
	}
	n, err := parse(v)
	if err != nil {
		return nil, err
	}
	return func(c *checked) bool {
		x := get(c)
		switch op {
		case ">":
			return x > n
		case ">=":
			return x >= n
		case "<":
			return x < n
		case "<=":
			return x <= n
		}
		return x == n
	}, nil
}

func parseSize(s string) (int64, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	mult := int64(1)
	for _, u := range []struct {
		suffix string
		mult   int64
	}{{"kb", 1 << 10}, {"mb", 1 << 20}, {"gb", 1 << 30}, {"k", 1 << 10}, {"m", 1 << 20}, {"b", 1}} {
		if strings.HasSuffix(s, u.suffix) {
			s, mult = strings.TrimSuffix(s, u.suffix), u.mult
			break
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("a size is like 512kb or 1mb")
	}
	return int64(f * float64(mult)), nil
}

func parseMillis(s string) (int64, error) {
	if n, err := strconv.Atoi(s); err == nil {
		return int64(n), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("a time is like 500ms or 2s")
	}
	return d.Milliseconds(), nil
}

func freeText(c *checked, text string) bool {
	if strings.Contains(strings.ToLower(c.sum.URL), text) || strings.Contains(strings.ToLower(c.sum.Method), text) {
		return true
	}
	d := c.detail()
	for _, hs := range [][]proto.Header{d.ReqHeaders, d.ResHeaders} {
		for _, h := range hs {
			if strings.Contains(strings.ToLower(h.Value), text) || strings.Contains(strings.ToLower(h.Name), text) {
				return true
			}
		}
	}
	return bytes.Contains(bytes.ToLower(c.body()), []byte(text))
}

// isGraphQL reports whether a request is a GraphQL operation: its path or
// its body says so.
func isGraphQL(c *checked) bool {
	if strings.Contains(c.sum.Path, "graphql") {
		return true
	}
	b := c.ex.rawBody("request")
	return bytes.Contains(b.data, []byte(`"query"`)) && bytes.Contains(b.data, []byte("{"))
}

// graphQLOp is a GraphQL request's operation name.
func graphQLOp(c *checked) string {
	b := c.ex.rawBody("request")
	data, _ := decode(b.data, b.encoding)
	var req struct {
		OperationName string `json:"operationName"`
		Query         string `json:"query"`
	}
	if json.Unmarshal(data, &req) != nil {
		return ""
	}
	if req.OperationName != "" {
		return req.OperationName
	}
	fields := strings.FieldsFunc(req.Query, func(r rune) bool { return unicode.IsSpace(r) || r == '{' || r == '(' })
	if len(fields) >= 2 && (fields[0] == "query" || fields[0] == "mutation" || fields[0] == "subscription") {
		return fields[1]
	}
	return ""
}

// splitFilter splits on spaces outside quotes.
func splitFilter(s string) []string {
	var out []string
	var cur strings.Builder
	quote := rune(0)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
			cur.WriteRune(r)
		case unicode.IsSpace(r):
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
