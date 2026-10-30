package hooks

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

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

// Matches reports whether hook h runs on event ev: on, its event, and
// every condition it sets met.
func Matches(h proto.Hook, ev proto.HookEvent) bool {
	if !h.On || h.Event != ev.Event {
		return false
	}
	if len(h.Agents) > 0 && !slices.Contains(h.Agents, ev.Agent) {
		return false
	}
	if len(h.Projects) > 0 && !slices.ContainsFunc(h.Projects, func(p string) bool { return sameProject(p, ev) }) {
		return false
	}
	if re, _ := compile(h.Tool); re != nil && !re.MatchString(ev.Tool) && !re.MatchString(ev.Command) {
		return false
	}
	if re, _ := compile(h.Text); re != nil && !re.MatchString(ev.Prompt) && !re.MatchString(ev.Command) {
		return false
	}
	if len(h.Paths) > 0 && !slices.ContainsFunc(ev.Paths, func(p string) bool {
		return slices.ContainsFunc(h.Paths, func(pattern string) bool { return projectlinks.Match(pattern, p) })
	}) {
		return false
	}
	return true
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
