package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/enowx/enowx/proto"
)

// Pausing and stopping the interceptor's recording (intercept.Capture):
// the state is this host's, the preferences the user's, synced with the
// other settings (settings/traffic.json).

func (s *Server) capturePrefsFile() string {
	return filepath.Join(s.dataDir, "settings", "traffic.json")
}

// loadCapturePrefs reads the preferences for pausing, defaults when none.
func (s *Server) loadCapturePrefs() proto.TrafficCapturePrefs {
	var p proto.TrafficCapturePrefs
	if b, err := os.ReadFile(s.capturePrefsFile()); err == nil {
		json.Unmarshal(b, &p)
	}
	if p.Rules == "" {
		p.Rules = "keep"
	}
	if p.Breakpoints == "" {
		p.Breakpoints = "release"
	}
	return p
}

// capturePrefsChanged hands the proxy the preferences as saved, after a
// sync brought them; nothing to do while it isn't running.
func (s *Server) capturePrefsChanged() {
	t := &s.traf
	t.mu.Lock()
	live := t.live
	t.mu.Unlock()
	if live {
		t.proxy.SetCapturePrefs(s.loadCapturePrefs())
	}
}

func (s *Server) trafficCapture() (proto.TrafficCapture, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficCapture{}, err
	}
	return t.proxy.Capture(), nil
}

func (s *Server) setTrafficCapture(p proto.TrafficCaptureParams) (proto.TrafficCapture, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficCapture{}, err
	}
	return t.proxy.SetCapture(p)
}

func (s *Server) capturePrefs() (proto.TrafficCapturePrefs, error) {
	return s.loadCapturePrefs(), nil
}

func (s *Server) setCapturePrefs(p proto.TrafficCapturePrefs) (proto.TrafficCapturePrefs, error) {
	if p.Rules != "pause" {
		p.Rules = "keep"
	}
	if p.Breakpoints != "drop" {
		p.Breakpoints = "release"
	}
	p.AutoMinutes, p.AutoCount = max(p.AutoMinutes, 0), max(p.AutoCount, 0)
	b, _ := json.MarshalIndent(p, "", "  ")
	if err := os.MkdirAll(filepath.Dir(s.capturePrefsFile()), 0o700); err != nil {
		return p, err
	}
	if err := writeFileAtomic(s.capturePrefsFile(), b); err != nil {
		return p, err
	}
	s.capturePrefsChanged()
	return p, nil
}
