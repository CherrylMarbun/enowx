package intercept

import (
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// The proxy records, pauses or stops (proto.TrafficCapture): paused, the
// traffic it pauses goes on unrecorded, its rules running or not; stopped,
// everything goes on as if there were no proxy. Its programs stay pointed
// here either way, so nothing they send fails.

// maxGaps is how many pauses and stops before are remembered.
const maxGaps = 50

// capture is the proxy's recording state; its fields belong to mu.
type capture struct {
	mu       sync.Mutex
	st       proto.TrafficCapture
	filter   *Filter
	recorded int // since recording began, for AutoCount
	prefs    proto.TrafficCapturePrefs
}

// Capture is how the proxy records now.
func (p *Proxy) Capture() proto.TrafficCapture {
	c := &p.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshot()
}

func (c *capture) snapshot() proto.TrafficCapture {
	st := c.st
	if st.State == "" {
		st.State = proto.CaptureRecording
	}
	st.Sources = slices.Clone(st.Sources)
	st.Gaps = slices.Clone(st.Gaps)
	return st
}

// SetCapturePrefs sets the defaults for pausing.
func (p *Proxy) SetCapturePrefs(prefs proto.TrafficCapturePrefs) {
	c := &p.capture
	c.mu.Lock()
	c.prefs = prefs
	c.mu.Unlock()
}

// SetCapture pauses, resumes, stops or starts recording as params say,
// releasing (or dropping) what waits at breakpoints when it pauses with
// the rules or stops; it answers the state then.
func (p *Proxy) SetCapture(params proto.TrafficCaptureParams) (proto.TrafficCapture, error) {
	var f *Filter
	if params.Filter != "" {
		var err error
		if f, err = ParseFilter(params.Filter); err != nil {
			return proto.TrafficCapture{}, err
		}
	}
	c := &p.capture
	c.mu.Lock()
	rules, breaks := params.Rules, params.Breakpoints
	if rules == "" {
		rules = c.prefs.Rules
	}
	if breaks == "" {
		breaks = c.prefs.Breakpoints
	}
	switch params.Action {
	case proto.CapturePause:
		c.begin(proto.TrafficCapture{State: proto.CapturePaused, Rules: rules, Sources: params.Sources, Filter: params.Filter}, f)
	case proto.CaptureStop:
		c.begin(proto.TrafficCapture{State: proto.CaptureStopped}, nil)
	case proto.CaptureResume, proto.CaptureStart:
		c.begin(proto.TrafficCapture{State: proto.CaptureRecording}, nil)
	default:
		c.mu.Unlock()
		return proto.TrafficCapture{}, errUnknownAction(params.Action)
	}
	st := c.snapshot()
	c.mu.Unlock()
	if st.State != proto.CaptureRecording {
		// What waits at a breakpoint won't be shown: it goes on.
		p.releaseAll(breaks == "drop")
	}
	p.captureChanged(st)
	return st, nil
}

// begin moves to state next, closing the gap of a pause or stop; c.mu is
// held.
func (c *capture) begin(next proto.TrafficCapture, f *Filter) {
	now := time.Now()
	if cur := c.st.State; cur == proto.CapturePaused || cur == proto.CaptureStopped {
		c.st.Gaps = append(c.st.Gaps, proto.TrafficGap{State: cur, From: c.st.Since, To: now, Skipped: c.st.Skipped,
			Partial: len(c.st.Sources) > 0 || c.st.Filter != ""})
		if len(c.st.Gaps) > maxGaps {
			c.st.Gaps = c.st.Gaps[len(c.st.Gaps)-maxGaps:]
		}
	}
	next.Since, next.Gaps = now, c.st.Gaps
	if next.State == proto.CaptureRecording {
		c.recorded = 0
	}
	c.st, c.filter = next, f
}

// decide says whether an exchange of src, as sum begins it, is recorded
// and goes through the rules, pausing on its own when the preferences say
// recording ran long enough.
func (p *Proxy) decide(src Source, sum proto.TrafficSummary) (record, rules bool) {
	c := &p.capture
	c.mu.Lock()
	var changed *proto.TrafficCapture
	if c.st.State == "" || c.st.State == proto.CaptureRecording {
		auto := ""
		switch {
		case c.prefs.AutoCount > 0 && c.recorded >= c.prefs.AutoCount:
			auto = "recorded " + strconv.Itoa(c.recorded) + " exchanges"
		case c.prefs.AutoMinutes > 0 && !c.st.Since.IsZero() && time.Since(c.st.Since) >= time.Duration(c.prefs.AutoMinutes)*time.Minute:
			auto = "recorded for " + strconv.Itoa(c.prefs.AutoMinutes) + " minutes"
		}
		if auto != "" {
			rules := c.prefs.Rules
			c.begin(proto.TrafficCapture{State: proto.CapturePaused, Rules: rules, Reason: "paused on its own: " + auto}, nil)
			st := c.snapshot()
			changed = &st
		}
	}
	switch c.st.State {
	case proto.CaptureStopped:
		record, rules = false, false
	case proto.CapturePaused:
		record, rules = true, true
		if c.pausesLocked(src, sum) {
			record, rules = false, c.st.Rules != "pause"
		}
	default:
		record, rules = true, true
	}
	if record {
		c.recorded++
	} else {
		c.st.Skipped++
	}
	c.mu.Unlock()
	if changed != nil {
		p.captureChanged(*changed)
	}
	return record, rules
}

// pausesLocked reports whether the pause takes in an exchange of src as
// sum begins it; c.mu is held.
func (c *capture) pausesLocked(src Source, sum proto.TrafficSummary) bool {
	if len(c.st.Sources) > 0 && !slices.ContainsFunc(c.st.Sources, func(s string) bool {
		return s == src.ID || s == src.Kind || s != "" && s == src.Session
	}) {
		return false
	}
	return c.filter == nil || c.filter.Match(&Exchange{sum: sum})
}

// capture is where an exchange of src, as sum begins it, is recorded,
// and whether rules apply to it.
func (p *Proxy) captureFor(src Source, sum proto.TrafficSummary) (*Exchange, bool) {
	record, rules := p.decide(src, sum)
	store := p.storeFor(src)
	if !record {
		store = discarded
	}
	return store.begin(sum), rules
}

// releaseAll lets every exchange waiting at a breakpoint go on, or drops
// them.
func (p *Proxy) releaseAll(drop bool) {
	p.mu.Lock()
	held := make([]*pause, 0, len(p.pauses))
	for _, ps := range p.pauses {
		held = append(held, ps)
	}
	p.mu.Unlock()
	for _, ps := range held {
		ps.resolve(resume{drop: drop})
	}
}

func (p *Proxy) captureChanged(st proto.TrafficCapture) {
	if p.OnCapture != nil {
		p.OnCapture(st)
	}
}

type errUnknownAction string

func (e errUnknownAction) Error() string {
	return "intercept: no capture action " + string(e) + ": pause, resume, stop or start"
}
