package daemon

import (
	"crypto/tls"
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
	hosts := map[string]string{}
	for k, v := range st.Hosts {
		hosts[strings.ToLower(k)] = v
	}
	p.Resolve = func(host string) string { return hosts[strings.ToLower(host)] }
	type certFor struct {
		pattern string
		cert    *tls.Certificate
	}
	var certs []certFor
	for _, c := range st.ClientCerts {
		cert, err := tls.LoadX509KeyPair(c.Cert, c.Key)
		if err != nil {
			return fmt.Errorf("traffic: the client certificate for %s: %w", c.Host, err)
		}
		certs = append(certs, certFor{c.Host, &cert})
	}
	p.ClientCert = func(host string) *tls.Certificate {
		for _, c := range certs {
			if intercept.HostMatches(c.pattern, host) {
				return c.cert
			}
		}
		return nil
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

// newTrafficCA makes the host's CA anew, and the files that trust it.
func (s *Server) newTrafficCA() (proto.TrafficCA, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficCA{}, err
	}
	ca, err := intercept.NewCA(t.dir)
	if err != nil {
		return proto.TrafficCA{}, err
	}
	// The same files, written anew: what points at them trusts the new CA.
	if _, err := intercept.WriteTrust(t.dir, ca); err != nil {
		return proto.TrafficCA{}, err
	}
	t.proxy.SetCA(ca)
	return s.trafficCA()
}
