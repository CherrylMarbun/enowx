package hooks

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/core/projectlinks"
	"github.com/enowx/enowx/proto"
)

var (
	compiledMu sync.Mutex
	compiled   = map[string]*regexp.Regexp{}
)

// compile is expression expr, case-insensitive, kept for the next event;
// nil for "".
func compile(expr string) (*regexp.Regexp, error) {
	if expr == "" {
		return nil, nil
	}
	compiledMu.Lock()
	defer compiledMu.Unlock()
	if re, ok := compiled[expr]; ok {
		return re, nil
	}
	re, err := regexp.Compile("(?i)" + expr)
	if err != nil {
		return nil, err
	}
	compiled[expr] = re
	return re, nil
}

// Matches reports whether hook h runs on event ev: on, its event, and its
// conditions met: all of those it sets, or any with Any, each turned
// round when Not names it.
func Matches(h proto.Hook, ev proto.HookEvent) bool {
	if !h.On || h.Event != ev.Event {
		return false
	}
	var results []bool
	add := func(name string, set bool, ok func() bool) {
		if set {
			results = append(results, ok() != slices.Contains(h.Not, name))
		}
	}
	add("agents", len(h.Agents) > 0, func() bool { return slices.Contains(h.Agents, ev.Agent) })
	add("projects", len(h.Projects) > 0, func() bool {
		return slices.ContainsFunc(h.Projects, func(p string) bool { return sameProject(p, ev) })
	})
	add("hosts", len(h.Hosts) > 0, func() bool {
		return slices.ContainsFunc(h.Hosts, func(x string) bool { return strings.EqualFold(x, ev.Host) })
	})
	add("branches", len(h.Branches) > 0, func() bool {
		return slices.ContainsFunc(h.Branches, func(g string) bool { ok, _ := path.Match(g, ev.Branch); return ok })
	})
	add("tool", h.Tool != "", func() bool {
		re, _ := compile(h.Tool)
		return re != nil && (re.MatchString(ev.Tool) || re.MatchString(ev.Command))
	})
	add("text", h.Text != "", func() bool {
		re, _ := compile(h.Text)
		return re != nil && (re.MatchString(ev.Prompt) || re.MatchString(ev.Command) || re.MatchString(ev.Text))
	})
	add("paths", len(h.Paths) > 0, func() bool {
		return slices.ContainsFunc(ev.Paths, func(p string) bool {
			return slices.ContainsFunc(h.Paths, func(pattern string) bool { return projectlinks.Match(pattern, p) })
		})
	})
	if len(results) == 0 {
		return true
	}
	if h.Any {
		return slices.Contains(results, true)
	}
	return !slices.Contains(results, false)
}

// sameProject says p names the project of ev: its folder (or the
// session's), or the folder's name.
func sameProject(p string, ev proto.HookEvent) bool {
	p = strings.TrimSuffix(p, string(filepath.Separator))
	for _, dir := range []string{ev.Project, ev.Dir} {
		if dir != "" && (p == dir || p == filepath.Base(dir)) {
			return true
		}
	}
	return false
}

// For is the hooks of list that run on ev, in order.
func For(list []proto.Hook, ev proto.HookEvent) []proto.Hook {
	var out []proto.Hook
	for _, h := range list {
		if Matches(h, ev) {
			out = append(out, h)
		}
	}
	return out
}

// hours reads a span of the day, "09-18" or "22:00-06:30", as minutes
// from midnight; to before from spans midnight.
func hours(s string) (from, to int, err error) {
	a, b, ok := strings.Cut(strings.ReplaceAll(s, " ", ""), "-")
	if !ok {
		return 0, 0, fmt.Errorf("hours %q: write them as 09-18 or 22:00-06:00", s)
	}
	parse := func(t string) (int, error) {
		h, m, _ := strings.Cut(t, ":")
		hh, err := strconv.Atoi(h)
		if err != nil || hh < 0 || hh > 24 {
			return 0, fmt.Errorf("hours %q: %q is no hour", s, t)
		}
		mm := 0
		if m != "" {
			if mm, err = strconv.Atoi(m); err != nil || mm < 0 || mm > 59 {
				return 0, fmt.Errorf("hours %q: %q is no time", s, t)
			}
		}
		return hh*60 + mm, nil
	}
	if from, err = parse(a); err != nil {
		return
	}
	to, err = parse(b)
	return
}

// within says t falls in span s ("" is always).
func within(s string, t time.Time) bool {
	if s == "" {
		return true
	}
	from, to, err := hours(s)
	if err != nil {
		return false
	}
	m := t.Hour()*60 + t.Minute()
	if from <= to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

// Limiter keeps how often hooks ran, for their limits (Once, Max, Every,
// Hours). Safe from any goroutine.
type Limiter struct {
	mu    sync.Mutex
	count map[string]int       // hook ID + session → runs
	last  map[string]time.Time // hook ID → its last run
}

// Take reports whether hook h may run on ev at now, and counts the run
// when it may.
func (l *Limiter) Take(h proto.Hook, ev proto.HookEvent, now time.Time) bool {
	if !within(h.Hours, now) {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.count == nil {
		l.count, l.last = map[string]int{}, map[string]time.Time{}
	}
	key := h.ID + "\x00" + ev.Session
	max := h.Max
	if h.Once {
		max = 1
	}
	if max > 0 && l.count[key] >= max {
		return false
	}
	if h.Every > 0 && now.Sub(l.last[h.ID]) < time.Duration(h.Every)*time.Second {
		return false
	}
	l.count[key]++
	l.last[h.ID] = now
	return true
}

// Forget drops the counts of session id, ended.
func (l *Limiter) Forget(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k := range l.count {
		if strings.HasSuffix(k, "\x00"+id) {
			delete(l.count, k)
		}
	}
}
