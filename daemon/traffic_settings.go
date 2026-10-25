package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/core/intercept"
	"github.com/enowx/enowx/proto"
)

func (s *Server) trafficSettingsFile() string { return filepath.Join(s.trafficDir(), "settings.json") }

// loadTrafficSettings reads the interceptor's settings, defaults when
// none were saved.
func (s *Server) loadTrafficSettings() proto.TrafficSettings {
	var st proto.TrafficSettings
	if b, err := os.ReadFile(s.trafficSettingsFile()); err == nil {
		json.Unmarshal(b, &st)
	}
	return st
}

// applyTrafficSettings sets the proxy up as st says.
func (s *Server) applyTrafficSettings(p *intercept.Proxy, st proto.TrafficSettings) error {
	insecure := append([]string(nil), st.Insecure...)
	p.Insecure = func(host string) bool {
		if host == "localhost" || strings.HasSuffix(host, ".localhost") {
			return true
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return true
		}
		for _, pat := range insecure {
			if intercept.HostMatches(pat, host) {
				return true
			}
		}
		return false
	}
	p.Upstream = nil
	if st.Upstream != "" {
		u, err := url.Parse(st.Upstream)
		if err != nil || u.Host == "" {
			return fmt.Errorf("traffic: the upstream proxy %q isn't a URL such as http://proxy:3128", st.Upstream)
		}
		p.Upstream = u
	}
	if st.MaxExchanges > 0 {
		p.Store.MaxExchanges = st.MaxExchanges
	}
	if st.MaxMB > 0 {
		p.Store.MaxBytes = int64(st.MaxMB) << 20
	}
	return nil
}

func (s *Server) trafficSettings() (proto.TrafficSettings, error) {
	if _, err := s.trafficOn(); err != nil {
		return proto.TrafficSettings{}, err
	}
	return s.loadTrafficSettings(), nil
}

// setTrafficSettings saves settings; the upstream proxy applies to
// connections made from then on.
func (s *Server) setTrafficSettings(st proto.TrafficSettings) (proto.TrafficSettings, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficSettings{}, err
	}
	if err := s.applyTrafficSettings(t.proxy, st); err != nil {
		return proto.TrafficSettings{}, err
	}
	t.proxy.ResetTransports()
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := writeFileAtomic(s.trafficSettingsFile(), b); err != nil {
		return proto.TrafficSettings{}, err
	}
	return st, nil
}
