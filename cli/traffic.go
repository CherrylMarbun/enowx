package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/enowx/enowx/core/intercept"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/textfmt"
)

const trafficUsage = `usage: enowx-app traffic <command>

  list [--filter F] [--all] [--limit N] [--json]   exchanges caught (this session's unless --all)
  show ID [--body request|response] [--json] [--as curl|fetch|python|go|httpie|powershell]
                                                   one exchange, bodies decoded, secrets masked
  watch [--filter F] [--until-match] [--timeout 1m] [--all]
                                                   print exchanges as they finish
  mock [match flags] [--status N] [--body TEXT|--body-file F] [--header 'K: V'] [--times N] [--delay MS]
                                                   answer matching requests from a mock
  breakpoint [match flags] [--stage request|response|both]
                                                   pause matching requests to edit them
  forward [match flags] --to ORIGIN [--keep-host]  send matching requests to another origin
  transform [match flags] [--req-set 'K: V'] [--req-remove K] [--res-set 'K: V'] [--res-remove K]
            [--res-status N] [--replace FROM=>TO] [--res-json a.b=VALUE] [--req-url URL]
  fail [match flags] --how 503|close|reset|timeout
  pass [match flags]                               let matching requests through, before later rules
            every rule: --name N --delay MS --throttle BYTES/S --project DIR
            mocks: --ws-message TEXT (repeat) --echo, for a WebSocket
  rules [--json]  |  rules rm|on|off ID  |  rules clear
  paused  |  resume ID [--drop] [--status N] [--body TEXT]   an exchange held at a breakpoint
  pause [--source S]... [--filter F] [--rules keep|pause] [--drop]   stop recording (traffic still flows)
  resume  |  stop [--drop]  |  start               record again; catch nothing at all; catch again
  prefs [--rules keep|pause] [--breakpoints release|drop] [--auto-minutes N] [--auto-count N]
  send [-X METHOD] [-H 'K: V']... [-d BODY] [--direct] URL
  clear [--filter F]
  export [--filter F] [--out FILE]                 HAR (stdout by default)
  import FILE                                      a HAR file
  attach terminal|session|manual|browser ID [--agent]   catch more of this session's traffic
  attached  |  detach SOURCE
  env [--off]                                      for eval "$(enowx-app traffic env)" in a shell
  status
  settings  |  settings set [--insecure H,H] [--upstream URL] [--host NAME=ADDR]... [--client-cert 'HOST CERT KEY']...
                            [--max-exchanges N] [--max-mb N] [--notify-errors on|off]
  ca [--pem] [--new]                               the CA's fingerprint and files; --new makes it anew
  pin ID...  |  unpin ID...                        kept when the rest is cleared
  filters  |  filters add F  |  filters rm F       filters saved with the project

match flags: --method GET,POST --host H (*.example.com: subdomains, .example.com: both)
             --path P (a prefix; with * a glob) --query k=v --header-match 'K: V'
             --body-match TEXT --json-match a.b=value
F is the filter language: domain:example.com host:*.example.com status:4xx type:json
method:POST path:/api/* body:"text" size:>1mb time:>500ms since:10m is:mocked ... (-x negates)
`

func (r runner) traffic(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprint(r.out, trafficUsage)
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "list", "ls":
		return r.trafficList(rest)
	case "show":
		return r.trafficShow(rest)
	case "watch":
		return r.trafficWatch(rest)
	case "mock":
		return r.trafficAddRule(rest, proto.ActionMock)
	case "breakpoint":
		return r.trafficAddRule(rest, proto.ActionBreakpoint)
	case "forward":
		return r.trafficAddRule(rest, proto.ActionForward)
	case "transform":
		return r.trafficAddRule(rest, proto.ActionTransform)
	case "fail":
		return r.trafficAddRule(rest, proto.ActionFail)
	case "pass":
		return r.trafficAddRule(rest, proto.ActionPass)
	case "settings":
		return r.trafficSettings(rest)
	case "ca":
		return r.trafficCA(rest)
	case "pin", "unpin":
		return r.trafficPin(cmd == "pin", rest)
	case "filters":
		return r.trafficFilters(rest)
	case "rules":
		return r.trafficRules(rest)
	case "paused":
		return r.trafficPaused()
	case "resume":
		if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
			return r.trafficCapture(proto.CaptureResume, rest)
		}
		return r.trafficResume(rest)
	case "pause", "stop", "start":
		return r.trafficCapture(cmd, rest)
	case "prefs":
		return r.trafficPrefs(rest)
	case "send":
		return r.trafficSend(rest)
	case "clear":
		fs := flagSet("clear")
		filter := fs.String("filter", "", "")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		return r.c.TrafficClear(proto.TrafficClearParams{Filter: *filter})
	case "export":
		return r.trafficExport(rest)
	case "import":
		if len(rest) != 1 {
			return errors.New("usage: enowx-app traffic import FILE")
		}
		data, err := os.ReadFile(rest[0])
		if err != nil {
			return err
		}
		l, err := r.c.TrafficImport(data)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.out, "imported %d exchanges\n", len(l.Exchanges))
		return nil
	case "attach":
		return r.trafficAttach(rest)
	case "attached":
		ts, err := r.c.TrafficAttached()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
		for _, t := range ts {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Source, t.Kind, t.Label, t.Proxy)
		}
		return w.Flush()
	case "detach":
		if len(rest) != 1 {
			return errors.New("usage: enowx-app traffic detach SOURCE")
		}
		return r.c.TrafficDetach(rest[0])
	case "env":
		return r.trafficEnv(rest)
	case "status":
		st, err := r.c.TrafficStatus()
		if err != nil {
			return err
		}
		if st.Error != "" {
			return errors.New(st.Error)
		}
		fmt.Fprintf(r.out, "listening on 127.0.0.1:%d\nCA %s\n%d exchanges, %s of bodies\n", st.Port, st.Fingerprint, st.Exchanges, textfmt.Bytes(uint64(st.Bytes)))
		if cap, err := r.c.TrafficCapture(); err == nil {
			writeCapture(r.out, cap)
		}
		return nil
	}
	return fmt.Errorf("traffic: no command %q: enowx-app traffic help lists them", cmd)
}

func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// scope is the filter that keeps to this session's traffic, "" outside
// a session or with all.
func (r runner) scope(all bool) string {
	if all || os.Getenv(EnvSession) == "" {
		return ""
	}
	s, err := r.session("self")
	if err != nil {
		return ""
	}
	id := s.ID
	if s.Parent != "" {
		id = s.Parent
	}
	return "session:" + id
}

func joinFilter(a, b string) string { return strings.TrimSpace(a + " " + b) }

func (r runner) trafficList(args []string) error {
	fs := flagSet("list")
	filter := fs.String("filter", "", "")
	all := fs.Bool("all", false, "")
	limit := fs.Int("limit", 100, "")
	asJSON := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	f := joinFilter(r.scope(*all), *filter+" "+strings.Join(fs.Args(), " "))
	var all_ []proto.TrafficSummary
	var after int64
	for {
		l, err := r.c.TrafficList(proto.TrafficListParams{Filter: f, After: after})
		if err != nil {
			return err
		}
		all_ = append(all_, l.Exchanges...)
		if !l.More || len(l.Exchanges) == 0 {
			break
		}
		after = l.Exchanges[len(l.Exchanges)-1].ID
	}
	if *limit > 0 && len(all_) > *limit {
		all_ = all_[len(all_)-*limit:]
	}
	if *asJSON {
		return json.NewEncoder(r.out).Encode(all_)
	}
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	for _, s := range all_ {
		writeSummary(w, s)
	}
	return w.Flush()
}

func writeSummary(w io.Writer, s proto.TrafficSummary) {
	status := strconv.Itoa(s.Status)
	switch {
	case s.State == proto.TrafficFailed:
		status = "ERR"
	case s.State == proto.TrafficHeld:
		status = "PAUSED"
	case s.State == proto.TrafficPending:
		status = "..."
	}
	notes := []string{}
	if s.Mocked {
		notes = append(notes, "mocked")
	} else if s.Rule != "" {
		notes = append(notes, "rule "+s.Rule)
	}
	if s.Error != "" {
		notes = append(notes, s.Error)
	}
	if s.Messages > 0 {
		notes = append(notes, strconv.Itoa(s.Messages)+" messages")
	}
	typ := s.Type
	if s.Kind != proto.TrafficHTTP {
		typ = s.Kind
	}
	fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t%s\t%s\t%dms\t%s\n", s.ID, status, s.Method, shorten(s.Host+s.Path, 90), typ,
		textfmt.Bytes(uint64(s.ResSize)), s.Duration, strings.Join(notes, ", "))
}

func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimPrefix(s, "#"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q isn't an exchange's number", s)
	}
	return id, nil
}

func (r runner) trafficShow(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: enowx-app traffic show ID [--body request|response] [--json]")
	}
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	fs := flagSet("show")
	which := fs.String("body", "", "")
	asJSON := fs.Bool("json", false, "")
	as := fs.String("as", "", "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *as != "" {
		// As code that sends it again: secrets masked, as everywhere here.
		ex, err := r.c.TrafficGet(id, false)
		if err != nil {
			return err
		}
		req, _ := r.c.TrafficBody(proto.TrafficBodyParams{ID: id, Which: "request", Size: 4 << 20})
		code, err := intercept.Snippet(*as, ex.Method, ex.URL, ex.ReqHeaders, req.Data)
		if err != nil {
			return err
		}
		fmt.Fprintln(r.out, code)
		return nil
	}
	if *which != "" {
		b, err := r.c.TrafficBody(proto.TrafficBodyParams{ID: id, Which: *which, Size: 4 << 20})
		if err != nil {
			return err
		}
		r.out.Write(b.Data)
		if b.Total > int64(len(b.Data)) {
			fmt.Fprintf(r.out, "\n… %s more\n", textfmt.Bytes(uint64(b.Total-int64(len(b.Data)))))
		}
		return nil
	}
	ex, err := r.c.TrafficGet(id, false)
	if err != nil {
		return err
	}
	req, _ := r.c.TrafficBody(proto.TrafficBodyParams{ID: id, Which: "request", Size: 64 << 10})
	res, _ := r.c.TrafficBody(proto.TrafficBodyParams{ID: id, Which: "response", Size: 64 << 10})
	var msgs []proto.TrafficMessage
	if ex.Messages > 0 {
		msgs, _ = r.c.TrafficMessages(proto.TrafficMessagesParams{ID: id, Limit: 200})
	}
	if *asJSON {
		return json.NewEncoder(r.out).Encode(map[string]any{
			"exchange": ex, "requestBody": bodyText(req), "responseBody": bodyText(res), "messages": msgs,
		})
	}
	fmt.Fprintf(r.out, "%s %s %s\n", ex.Method, ex.URL, ex.Proto)
	for _, h := range ex.ReqHeaders {
		fmt.Fprintf(r.out, "%s: %s\n", h.Name, h.Value)
	}
	if t := bodyText(req); t != "" {
		fmt.Fprintf(r.out, "\n%s\n", t)
	}
	fmt.Fprintln(r.out, "\n──")
	if ex.Error != "" {
		fmt.Fprintf(r.out, "error: %s\n", ex.Error)
	}
	if ex.Status != 0 {
		fmt.Fprintf(r.out, "%d (%dms", ex.Status, ex.Duration)
		if ex.Rule != "" {
			fmt.Fprintf(r.out, ", rule %s", ex.Rule)
		}
		fmt.Fprintln(r.out, ")")
	}
	for _, h := range ex.ResHeaders {
		fmt.Fprintf(r.out, "%s: %s\n", h.Name, h.Value)
	}
	if t := bodyText(res); t != "" {
		fmt.Fprintf(r.out, "\n%s\n", t)
	}
	for _, m := range msgs {
		dir := "←"
		if m.Sent {
			dir = "→"
		}
		text := string(m.Data)
		if pb, ok := intercept.DecodeProtobuf(m.Data); ok && (m.Kind == "message" || !utf8.Valid(m.Data)) {
			text = strings.ReplaceAll(strings.TrimSpace(pb), "\n", "; ")
		}
		fmt.Fprintf(r.out, "%s %s %s\n", dir, m.Kind, shorten(text, 400))
	}
	return nil
}

// bodyText is a body as text, a note for binary ones.
func bodyText(b proto.TrafficBody) string {
	if len(b.Data) == 0 {
		return ""
	}
	if !utf8.Valid(b.Data) {
		return fmt.Sprintf("(%s of binary data, %s)", textfmt.Bytes(uint64(b.Total)), b.ContentType)
	}
	s := string(b.Data)
	if b.Total > int64(len(b.Data)) {
		s += fmt.Sprintf("\n… %s more (traffic show ID --body …)", textfmt.Bytes(uint64(b.Total-int64(len(b.Data)))))
	}
	return s
}

func (r runner) trafficWatch(args []string) error {
	fs := flagSet("watch")
	filter := fs.String("filter", "", "")
	until := fs.Bool("until-match", false, "")
	timeout := fs.Duration("timeout", 0, "")
	all := fs.Bool("all", false, "")
	asJSON := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	f := joinFilter(r.scope(*all), *filter+" "+strings.Join(fs.Args(), " ")+" -is:pending -is:paused")
	// From now on: what was caught before isn't news.
	var after int64
	if l, err := r.c.TrafficList(proto.TrafficListParams{}); err == nil {
		for _, s := range l.Exchanges {
			after = max(after, s.ID)
		}
		for l.More {
			if l, err = r.c.TrafficList(proto.TrafficListParams{After: after}); err != nil {
				break
			}
			for _, s := range l.Exchanges {
				after = max(after, s.ID)
			}
		}
	}
	var deadline time.Time
	if *timeout > 0 {
		deadline = time.Now().Add(*timeout)
	}
	seen := map[int64]bool{}
	told := ""
	for {
		// Paused, nothing new shows: say so rather than wait in silence.
		if st, err := r.c.TrafficCapture(); err == nil {
			if line := capturePaused(st); line != told {
				if line != "" && !*asJSON {
					fmt.Fprintln(r.out, line)
				}
				told = line
			}
		}
		l, err := r.c.TrafficList(proto.TrafficListParams{Filter: f, After: after})
		if err != nil {
			return err
		}
		for _, s := range l.Exchanges {
			if seen[s.ID] {
				continue
			}
			seen[s.ID] = true
			if *asJSON {
				json.NewEncoder(r.out).Encode(s)
			} else {
				w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
				writeSummary(w, s)
				w.Flush()
			}
			if *until {
				return nil
			}
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			if *until {
				return errors.New("traffic: nothing matched before the timeout")
			}
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// matchFlags adds the flags that say what a rule matches.
func matchFlags(fs *flag.FlagSet) func() (proto.TrafficMatch, error) {
	method := fs.String("method", "", "")
	host := fs.String("host", "", "")
	path := fs.String("path", "", "")
	query := fs.String("query", "", "")
	header := fs.String("header-match", "", "")
	body := fs.String("body-match", "", "")
	jsonMatch := fs.String("json-match", "", "")
	return func() (proto.TrafficMatch, error) {
		m := proto.TrafficMatch{Host: *host, Path: *path, Body: *body}
		if *method != "" {
			m.Methods = strings.Split(strings.ToUpper(*method), ",")
		}
		if strings.ContainsAny(*path, "*?") {
			m.PathMode = "glob"
		}
		if *query != "" {
			k, v, _ := strings.Cut(*query, "=")
			m.Query = map[string]string{k: v}
		}
		if *header != "" {
			k, v, _ := strings.Cut(*header, ":")
			m.Headers = map[string]string{strings.TrimSpace(k): strings.TrimSpace(v)}
		}
		if *jsonMatch != "" {
			k, v, _ := strings.Cut(*jsonMatch, "=")
			m.JSON = map[string]string{k: v}
		}
		if m.Host == "" && m.Path == "" && len(m.Methods) == 0 && m.Body == "" && m.Query == nil && m.Headers == nil && m.JSON == nil {
			return m, errors.New("say what to match: --host, --path, --method, …")
		}
		return m, nil
	}
}

// project is the project whose rules the command changes: --project, or
// this session's.
func (r runner) project(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if s, err := r.session("self"); err == nil {
		return s.Project
	}
	return ""
}

func (r runner) trafficAddRule(args []string, kind string) error {
	fs := flagSet(kind)
	match := matchFlags(fs)
	project := fs.String("project", "", "")
	name := fs.String("name", "", "")
	status := fs.Int("status", 0, "")
	body := fs.String("body", "", "")
	bodyFile := fs.String("body-file", "", "")
	var headers multiFlag
	fs.Var(&headers, "header", "")
	times := fs.Int("times", 0, "")
	delay := fs.Int("delay", 0, "")
	throttle := fs.Int("throttle", 0, "")
	stage := fs.String("stage", "request", "")
	var wsMessages multiFlag
	fs.Var(&wsMessages, "ws-message", "")
	echo := fs.Bool("echo", false, "")
	to := fs.String("to", "", "")
	keepHost := fs.Bool("keep-host", false, "")
	how := fs.String("how", "status", "")
	tf := transformFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	m, err := match()
	if err != nil {
		return err
	}
	a := proto.TrafficAction{Kind: kind, DelayMs: *delay, Throttle: *throttle}
	switch kind {
	case proto.ActionMock:
		a.Status, a.Body, a.BodyFile, a.Times, a.Messages, a.Echo = *status, *body, *bodyFile, *times, wsMessages, *echo
		a.Headers = headerList(headers)
	case proto.ActionBreakpoint:
		a.Break = *stage
	case proto.ActionForward:
		if *to == "" {
			return errors.New("usage: enowx-app traffic forward [match flags] --to https://staging.example.com [--keep-host]")
		}
		a.Target, a.KeepHost = *to, *keepHost
	case proto.ActionFail:
		a.Fail = map[string]string{"503": "status", "5xx": "status"}[*how]
		if a.Fail == "" {
			a.Fail = *how
		}
		if !slices.Contains([]string{"status", "close", "reset", "timeout"}, a.Fail) {
			return errors.New("traffic: --how is 503, close, reset or timeout")
		}
	case proto.ActionTransform:
		if a.Request, a.Response, err = tf(); err != nil {
			return err
		}
	}
	proj := r.project(*project)
	rules, err := r.c.TrafficRules(proj)
	if err != nil {
		return err
	}
	// New rules go first: the most specific is usually the latest.
	rules.Rules = append([]proto.TrafficRule{{Name: *name, Enabled: true, Match: m, Action: a}}, rules.Rules...)
	rules.Project = proj
	saved, err := r.c.SetTrafficRules(rules)
	if err != nil {
		return err
	}
	for _, rl := range saved.Rules {
		if rl.Problem != "" {
			return fmt.Errorf("the rule can't apply: %s", rl.Problem)
		}
	}
	fmt.Fprintln(r.out, saved.Rules[0].ID)
	return nil
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ", ") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func (r runner) trafficRules(args []string) error {
	fs := flagSet("rules")
	project := fs.String("project", "", "")
	asJSON := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	proj := r.project(*project)
	rules, err := r.c.TrafficRules(proj)
	if err != nil {
		return err
	}
	if fs.NArg() > 0 {
		cmd := fs.Arg(0)
		if cmd == "clear" {
			rules.Rules = nil
		} else {
			if fs.NArg() < 2 {
				return errors.New("usage: enowx-app traffic rules rm|on|off ID")
			}
			id := fs.Arg(1)
			found := false
			for i := range rules.Rules {
				if rules.Rules[i].ID != id {
					continue
				}
				found = true
				switch cmd {
				case "rm":
					rules.Rules = append(rules.Rules[:i], rules.Rules[i+1:]...)
				case "on", "off":
					rules.Rules[i].Enabled = cmd == "on"
				default:
					return fmt.Errorf("rules: no command %q", cmd)
				}
				break
			}
			if !found {
				return fmt.Errorf("no rule %s", id)
			}
		}
		rules.Project = proj
		_, err := r.c.SetTrafficRules(rules)
		return err
	}
	if *asJSON {
		return json.NewEncoder(r.out).Encode(rules)
	}
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	for _, rl := range rules.Rules {
		on := "on"
		if !rl.Enabled {
			on = "off"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d hits\t%s\n", rl.ID, on, intercept.Describe(rl), rl.Hits, rl.Problem)
	}
	return w.Flush()
}

func (r runner) trafficPaused() error {
	ps, err := r.c.TrafficPaused()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	for _, p := range ps {
		fmt.Fprintf(w, "#%d\t%s\t%s %s\t%s ago\n", p.ID, p.Stage, p.Method, p.URL, time.Since(p.Since).Round(time.Second))
	}
	return w.Flush()
}

func (r runner) trafficResume(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: enowx-app traffic resume ID [--drop] [--status N] [--body TEXT]")
	}
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	fs := flagSet("resume")
	drop := fs.Bool("drop", false, "")
	status := fs.Int("status", 0, "")
	body := fs.String("body", "\x00", "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	p := proto.TrafficResumeParams{ID: id, Drop: *drop}
	if *status != 0 || *body != "\x00" {
		p.Edit = &proto.TrafficEdit{Status: *status}
		if *body != "\x00" {
			b := []byte(*body)
			p.Edit.Body = &b
		}
	}
	return r.c.TrafficResume(p)
}

func (r runner) trafficSend(args []string) error {
	fs := flagSet("send")
	method := fs.String("X", "", "")
	data := fs.String("d", "", "")
	direct := fs.Bool("direct", false, "")
	var headers multiFlag
	fs.Var(&headers, "H", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		return errors.New("usage: enowx-app traffic send [-X METHOD] [-H 'K: V'] [-d BODY] [--direct] URL")
	}
	p := proto.TrafficSendParams{Method: *method, URL: fs.Arg(0), Body: []byte(*data), Direct: *direct, Project: r.project("")}
	if p.Method == "" {
		p.Method = "GET"
		if *data != "" {
			p.Method = "POST"
		}
	}
	for _, h := range headers {
		k, v, _ := strings.Cut(h, ":")
		p.Headers = append(p.Headers, proto.Header{Name: strings.TrimSpace(k), Value: strings.TrimSpace(v)})
	}
	if s, err := r.session("self"); err == nil {
		p.Session = s.ID
	}
	sum, err := r.c.TrafficSend(p)
	if err != nil {
		return err
	}
	return r.trafficShow([]string{strconv.FormatInt(sum.ID, 10)})
}

func (r runner) trafficExport(args []string) error {
	fs := flagSet("export")
	filter := fs.String("filter", "", "")
	outFile := fs.String("out", "", "")
	all := fs.Bool("all", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	har, err := r.c.TrafficExport(proto.TrafficExportParams{Filter: joinFilter(r.scope(*all), *filter)})
	if err != nil {
		return err
	}
	if *outFile == "" {
		_, err := r.out.Write(har)
		return err
	}
	if err := os.WriteFile(*outFile, har, 0o600); err != nil {
		return err
	}
	fmt.Fprintln(r.out, *outFile)
	return nil
}

func (r runner) trafficAttach(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: enowx-app traffic attach terminal|session|docker|manual|browser NAME|electron PATH|android [SERIAL]|webview [--agent] [--session S]")
	}
	switch args[0] {
	case proto.TargetAndroid:
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			return r.androidCatch(append([]string{"-s", args[1]}, args[2:]...))
		}
		return r.androidCatch(args[1:])
	case proto.TargetWebView, "browser-tool":
		return r.browser(append(args[1:], proto.BrowserCapture, "on"))
	}
	fs := flagSet("attach")
	session := fs.String("session", "self", "")
	agentToo := fs.Bool("agent", false, "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	p := proto.TrafficAttachParams{Kind: args[0], Agent: *agentToo}
	if s, err := r.session(*session); err == nil {
		p.Session = s.ID
		if s.Parent != "" {
			p.Session = s.Parent
		}
	}
	if p.Kind == proto.TargetBrowser || p.Kind == proto.TargetElectron {
		if fs.NArg() != 1 {
			return errors.New("usage: enowx-app traffic attach browser chrome|chromium|edge|brave|firefox, or electron PATH (an executable, or a macOS .app)")
		}
		p.Program = fs.Arg(0)
	}
	t, err := r.c.TrafficAttach(p)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "%s\t%s\n", t.Source, t.Proxy)
	if t.Note != "" {
		fmt.Fprintln(r.out, t.Note)
	}
	return nil
}

func (r runner) trafficEnv(args []string) error {
	fs := flagSet("env")
	off := fs.Bool("off", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p := proto.TrafficEnvParams{Source: os.Getenv("ENOWX_TRAFFIC_SOURCE")}
	if s, err := r.session("self"); err == nil {
		p.Session = s.ID
		if s.Parent != "" {
			p.Session = s.Parent
		}
	}
	env, err := r.c.TrafficEnv(p)
	if err != nil {
		return err
	}
	if *off {
		fmt.Fprint(r.out, env.Unset)
		return nil
	}
	fmt.Fprint(r.out, env.Shell)
	return nil
}
