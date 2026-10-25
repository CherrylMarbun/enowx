package daemon

import (
	"errors"
	"net"
	"strconv"

	"github.com/enowx/enowx/proto"
)

// Targets that come with their own machinery: Android devices (the
// mobile tool) and Docker containers.

// reverse puts a reverse proxy in front of a session's port, its traffic
// recorded as the session's Browser tool's (a webview target, made the
// first time).
func (s *Server) reverse(p proto.TrafficReverseParams) (proto.TrafficReverse, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficReverse{}, err
	}
	key := p.Session + ":" + strconv.Itoa(p.Port)
	t.mu.Lock()
	if ln, ok := t.revs[key]; ok {
		src := t.revSources[key]
		t.mu.Unlock()
		return proto.TrafficReverse{Port: ln.Addr().(*net.TCPAddr).Port, Source: src}, nil
	}
	var tg *trafficTarget
	for _, x := range t.targets {
		if x.Kind == proto.TargetWebView && x.Session == p.Session && !x.Src.Quiet {
			tg = x
		}
	}
	t.mu.Unlock()
	if tg == nil {
		if tg, err = s.addTarget(proto.TargetWebView, "Browser", p.Session, false); err != nil {
			return proto.TrafficReverse{}, err
		}
		s.targetsChanged()
	}
	ln, err := t.proxy.Reverse(tg.Src, "127.0.0.1:"+strconv.Itoa(p.Port))
	if err != nil {
		return proto.TrafficReverse{}, err
	}
	t.mu.Lock()
	if t.revs == nil {
		t.revs, t.revSources = map[string]net.Listener{}, map[string]string{}
	}
	t.revs[key], t.revSources[key] = ln, tg.Source
	t.mu.Unlock()
	return proto.TrafficReverse{Port: ln.Addr().(*net.TCPAddr).Port, Source: tg.Source}, nil
}

// closeReverses closes source's reverse proxies.
func (s *Server) closeReverses(source string) {
	t := &s.traf
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, src := range t.revSources {
		if src == source {
			t.revs[key].Close()
			delete(t.revs, key)
			delete(t.revSources, key)
		}
	}
}

// trafficTunnel tunnels to a source's own port, for a device on another
// host.
func (c *conn) trafficTunnel(p proto.TrafficTargetRef) (proto.TunnelInfo, error) {
	t, err := c.srv.trafficOn()
	if err != nil {
		return proto.TunnelInfo{}, err
	}
	t.mu.Lock()
	tg := t.targets[p.Source]
	t.mu.Unlock()
	if tg == nil {
		return proto.TunnelInfo{}, errors.New("traffic: no such source")
	}
	port := tg.Port
	if p.Port != 0 {
		// One of its reverse proxies, and no other port.
		port = 0
		t.mu.Lock()
		for key, src := range t.revSources {
			if ln := t.revs[key]; src == p.Source && ln.Addr().(*net.TCPAddr).Port == p.Port {
				port = p.Port
			}
		}
		t.mu.Unlock()
		if port == 0 {
			return proto.TunnelInfo{}, errors.New("traffic: no such reverse proxy")
		}
	}
	nc, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return proto.TunnelInfo{}, err
	}
	id, err := c.attachTunnel(nc)
	return proto.TunnelInfo{Stream: id}, err
}

// androidProxy points a device here at a port of this host, with the
// interceptor's CA.
func (s *Server) androidProxy(p proto.AndroidProxyParams) (proto.AndroidProxyResult, error) {
	sdk, err := s.androidSDK()
	if err != nil {
		return proto.AndroidProxyResult{}, err
	}
	if p.Off {
		return proto.AndroidProxyResult{}, sdk.ProxyOff(p.Serial, p.Port)
	}
	note, err := sdk.Proxy(p.Serial, p.Port, p.CA)
	return proto.AndroidProxyResult{Note: note}, err
}
