package intercept

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Describe says a rule in a few words: what it matches, then what it
// does.
func Describe(r proto.TrafficRule) string {
	m := r.Match
	what := strings.Join(m.Methods, ",")
	if what == "" {
		what = "any"
	}
	where := m.Host + m.Path
	if where == "" {
		where = "*"
	}
	s := what + " " + where
	if m.Body != "" {
		s += " with " + strconv.Quote(m.Body)
	}
	a := r.Action
	switch a.Kind {
	case proto.ActionMock:
		st := a.Status
		if st == 0 {
			st = 200
		}
		s += fmt.Sprintf(" → mock %d", st)
		if a.Times > 0 {
			s += fmt.Sprintf(" (%d times)", a.Times)
		}
	case proto.ActionForward:
		s += " → forward to " + a.Target
	case proto.ActionTransform:
		s += " → transform"
	case proto.ActionFail:
		s += " → fail (" + a.Fail + ")"
	case proto.ActionBreakpoint:
		s += " → pause the " + a.Break
	default:
		s += " → pass"
	}
	if a.DelayMs > 0 {
		s += fmt.Sprintf(", delay %dms", a.DelayMs)
	}
	if a.Throttle > 0 {
		s += ", throttled to " + strconv.Itoa(a.Throttle) + " B" + "/s"
	}
	return s
}
