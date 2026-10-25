package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

const captureUsage = `usage: enowx-app traffic pause [--source S]... [--filter F] [--rules keep|pause] [--drop]
       enowx-app traffic resume | stop [--drop] | start | status
       enowx-app traffic prefs [--rules keep|pause] [--breakpoints release|drop] [--auto-minutes N] [--auto-count N]`

// trafficCapture pauses, resumes, stops or starts recording (cmd), with
// pause's flags.
func (r runner) trafficCapture(cmd string, args []string) error {
	fs := flagSet(cmd)
	var sources multiFlag
	fs.Var(&sources, "source", "")
	filter := fs.String("filter", "", "")
	rules := fs.String("rules", "", "")
	drop := fs.Bool("drop", false, "")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		return fmt.Errorf("%s", captureUsage)
	}
	p := proto.TrafficCaptureParams{Action: cmd, Sources: splitList(sources), Filter: *filter, Rules: *rules}
	if *drop {
		p.Breakpoints = "drop"
	}
	st, err := r.c.SetTrafficCapture(p)
	if err != nil {
		return err
	}
	writeCapture(r.out, st)
	return nil
}

// trafficPrefs shows or sets the defaults for pausing.
func (r runner) trafficPrefs(args []string) error {
	fs := flagSet("prefs")
	rules := fs.String("rules", "", "")
	breaks := fs.String("breakpoints", "", "")
	minutes := fs.Int("auto-minutes", -1, "")
	count := fs.Int("auto-count", -1, "")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%s", captureUsage)
	}
	p, err := r.c.TrafficCapturePrefs()
	if err != nil {
		return err
	}
	if fs.NFlag() > 0 {
		if *rules != "" {
			p.Rules = *rules
		}
		if *breaks != "" {
			p.Breakpoints = *breaks
		}
		if *minutes >= 0 {
			p.AutoMinutes = *minutes
		}
		if *count >= 0 {
			p.AutoCount = *count
		}
		if p, err = r.c.SetTrafficCapturePrefs(p); err != nil {
			return err
		}
	}
	fmt.Fprintf(r.out, "while paused, rules: %s\nheld at breakpoints when pausing: %s\n", p.Rules, p.Breakpoints)
	if p.AutoMinutes > 0 {
		fmt.Fprintf(r.out, "pauses on its own after %d minutes of recording\n", p.AutoMinutes)
	}
	if p.AutoCount > 0 {
		fmt.Fprintf(r.out, "pauses on its own after %d exchanges\n", p.AutoCount)
	}
	return nil
}

// writeCapture says how the interceptor records.
func writeCapture(w io.Writer, st proto.TrafficCapture) {
	since := st.Since.Local().Format("15:04:05")
	switch st.State {
	case proto.CapturePaused:
		what := "paused"
		if len(st.Sources) > 0 {
			what += " for " + strings.Join(st.Sources, ", ")
		}
		if st.Filter != "" {
			what += " where " + st.Filter
		}
		rules := "rules running"
		if st.Rules == "pause" {
			rules = "rules paused"
		}
		fmt.Fprintf(w, "%s since %s (%s), %d not recorded\n", what, since, rules, st.Skipped)
		if st.Reason != "" {
			fmt.Fprintln(w, st.Reason)
		}
	case proto.CaptureStopped:
		fmt.Fprintf(w, "stopped since %s: traffic goes through untouched, %d not recorded\n", since, st.Skipped)
	default:
		fmt.Fprintln(w, "recording")
	}
	for _, g := range st.Gaps[max(0, len(st.Gaps)-3):] {
		fmt.Fprintf(w, "  %s %s–%s, %d not recorded\n", g.State, g.From.Local().Format("15:04"), g.To.Local().Format("15:04"), g.Skipped)
	}
}

// capturePaused is a line saying that recording is paused or stopped,
// "" while it records.
func capturePaused(st proto.TrafficCapture) string {
	if st.State == proto.CapturePaused || st.State == proto.CaptureStopped {
		return fmt.Sprintf("traffic: recording is %s since %s (\"enowx-app traffic resume\" or \"start\" records again)", st.State, st.Since.Local().Format(time.TimeOnly))
	}
	return ""
}

func splitList(vals []string) []string {
	var out []string
	for _, v := range vals {
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// captureTools are pausing and stopping the recording as MCP tools.
var captureTools = []mcpTool{
	{name: "traffic_pause", description: "Pause recording traffic: it still flows through enowX unrecorded. Optionally only some sources' (source IDs, kinds such as browser or terminal, session IDs) or a filter's (e.g. domain:analytics.example.com); rules keep running unless rules=pause.",
		props: map[string]mcpProp{"sources": {"array", "Sources to pause"}, "filter": {"string", "Pause only what this filter matches"}, "rules": {"string", "keep or pause"}},
		args: func(a map[string]any) []string {
			args := []string{"pause"}
			if list, ok := a["sources"].([]any); ok {
				for _, s := range list {
					args = append(args, "--source", fmt.Sprint(s))
				}
			}
			return withFlag(withFlag(args, a, "filter", "filter"), a, "rules", "rules")
		}},
	{name: "traffic_resume_recording", description: "Record traffic again after a pause.",
		args: func(map[string]any) []string { return []string{"resume"} }},
	{name: "traffic_stop", description: "Stop catching traffic: programs pointed at enowX go through untouched, nothing recorded, no rules, until traffic_start.",
		args: func(map[string]any) []string { return []string{"stop"} }},
	{name: "traffic_start", description: "Catch and record traffic again after a stop.",
		args: func(map[string]any) []string { return []string{"start"} }},
	{name: "traffic_status", description: "The interceptor: its port, what it keeps, and whether it records, is paused (for what) or stopped.",
		args: func(map[string]any) []string { return []string{"status"} }},
}
