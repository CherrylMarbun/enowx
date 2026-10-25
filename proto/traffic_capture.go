package proto

import "time"

// The interceptor records, or pauses (traffic still flows, unrecorded;
// rules keep running or pause too), or stops (traffic flows through as
// if there were no interceptor: nothing recorded, no rules). The state is
// the host's own; the defaults for pausing are synced with the user's
// other settings (settings/traffic.json).
const (
	MethodTrafficCapture         = "traffic.capture"           // → TrafficCapture
	MethodTrafficCaptureSet      = "traffic.capture.set"       // TrafficCaptureParams → TrafficCapture
	MethodTrafficCapturePrefs    = "traffic.capture.prefs"     // → TrafficCapturePrefs
	MethodTrafficCapturePrefsSet = "traffic.capture.prefs.set" // TrafficCapturePrefs → TrafficCapturePrefs
)

// EventTrafficCapture carries TrafficCapture after it changed.
const EventTrafficCapture = "traffic.capture"

// Capture states.
const (
	CaptureRecording = "recording"
	CapturePaused    = "paused"
	CaptureStopped   = "stopped"
)

// Capture actions (TrafficCaptureParams.Action).
const (
	CapturePause  = "pause"
	CaptureResume = "resume"
	CaptureStop   = "stop"
	CaptureStart  = "start"
)

// TrafficCapture is how the interceptor records now.
type TrafficCapture struct {
	State string    `json:"state"`
	Since time.Time `json:"since"`
	// While paused: Rules "pause" holds the rules too ("keep" runs them);
	// Sources, when set, pauses only the traffic of those sources (by
	// source ID, kind such as browser or terminal, or session ID), and
	// Filter only what it matches.
	Rules   string   `json:"rules,omitempty"`
	Sources []string `json:"sources,omitempty"`
	Filter  string   `json:"filter,omitempty"`
	// Skipped counts the exchanges not recorded since it paused or
	// stopped.
	Skipped int `json:"skipped"`
	// Reason says why it paused, when it paused itself.
	Reason string `json:"reason,omitempty"`
	// Gaps are the pauses and stops before, the latest last.
	Gaps []TrafficGap `json:"gaps,omitempty"`
}

// TrafficGap is a time the interceptor didn't record.
type TrafficGap struct {
	State   string    `json:"state"` // paused or stopped
	From    time.Time `json:"from"`
	To      time.Time `json:"to"`
	Skipped int       `json:"skipped"`
	Partial bool      `json:"partial,omitempty"` // only some sources or a filter
}

// TrafficCaptureParams pauses, resumes, stops or starts recording.
// Pausing takes Sources, Filter and Rules (as TrafficCapture's; Rules ""
// for the preference); pausing and stopping release the exchanges held at
// breakpoints, or with Breakpoints "drop", drop them ("" for the
// preference).
type TrafficCaptureParams struct {
	Action      string   `json:"action"`
	Sources     []string `json:"sources,omitempty"`
	Filter      string   `json:"filter,omitempty"`
	Rules       string   `json:"rules,omitempty"`
	Breakpoints string   `json:"breakpoints,omitempty"`
}

// TrafficCapturePrefs are the defaults for pausing, the user's on every
// host: whether rules keep running while paused, what becomes of
// exchanges held at breakpoints, and pausing on its own after AutoMinutes
// of recording or AutoCount exchanges recorded (0: never).
type TrafficCapturePrefs struct {
	Rules       string `json:"rules,omitempty"`       // keep (default) or pause
	Breakpoints string `json:"breakpoints,omitempty"` // release (default) or drop
	AutoMinutes int    `json:"autoMinutes,omitempty"`
	AutoCount   int    `json:"autoCount,omitempty"`
}
