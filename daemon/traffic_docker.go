package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

// Docker containers a session starts reach the interceptor through their
// host: a wrapper docker, first on the PATH of a tab made for it, gives
// docker run and create the proxy and the CA. On Linux the source also
// listens on the docker0 bridge's address, which host.docker.internal
// names there; Docker Desktop reaches the host's loopback by itself.

// containerCA is where the CA is mounted in containers.
const containerCA = "/etc/enowx-traffic"

func (s *Server) attachDocker(p proto.TrafficAttachParams) (proto.TrafficTarget, error) {
	if p.Session == "" {
		return proto.TrafficTarget{}, errors.New("traffic: containers are caught from a session")
	}
	if runtime.GOOS == "windows" {
		// The wrapper is a shell script.
		return proto.TrafficTarget{}, errors.New("traffic: Docker containers are caught on Linux and macOS hosts")
	}
	real, err := exec.LookPath("docker")
	if err != nil {
		return proto.TrafficTarget{}, errors.New("traffic: no docker on this host")
	}
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficTarget{}, err
	}
	tg, err := s.addTarget(proto.TargetDocker, "Docker", p.Session, false)
	if err != nil {
		return proto.TrafficTarget{}, err
	}
	port := tg.Port
	if runtime.GOOS == "linux" {
		gw, err := dockerGateway()
		if err != nil {
			s.detach(tg.Source)
			return proto.TrafficTarget{}, err
		}
		ln, err := t.proxy.SourceListen(tg.Src, net.JoinHostPort(gw, "0"))
		if err != nil {
			s.detach(tg.Source)
			return proto.TrafficTarget{}, err
		}
		port = ln.Addr().(*net.TCPAddr).Port
		t.mu.Lock()
		if t.revs == nil {
			t.revs, t.revSources = map[string]net.Listener{}, map[string]string{}
		}
		key := "docker:" + tg.Source
		t.revs[key], t.revSources[key] = ln, tg.Source
		t.mu.Unlock()
	}
	dir := filepath.Join(t.dir, "bin", tg.Source)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return proto.TrafficTarget{}, err
	}
	proxy := "http://host.docker.internal:" + strconv.Itoa(port)
	flags := []string{"--add-host=host.docker.internal:host-gateway",
		"-v", t.trust.Bundle + ":" + containerCA + "/bundle.pem:ro", "-v", t.trust.CA + ":" + containerCA + "/ca.pem:ro"}
	for _, kv := range []string{
		"HTTP_PROXY=" + proxy, "HTTPS_PROXY=" + proxy, "http_proxy=" + proxy, "https_proxy=" + proxy,
		"NO_PROXY=localhost,127.0.0.1", "no_proxy=localhost,127.0.0.1",
		"SSL_CERT_FILE=" + containerCA + "/bundle.pem", "REQUESTS_CA_BUNDLE=" + containerCA + "/bundle.pem",
		"CURL_CA_BUNDLE=" + containerCA + "/bundle.pem", "NODE_EXTRA_CA_CERTS=" + containerCA + "/ca.pem",
		"NODE_USE_ENV_PROXY=1",
	} {
		flags = append(flags, "-e", kv)
	}
	quoted := make([]string, len(flags))
	for i, f := range flags {
		quoted[i] = "'" + strings.ReplaceAll(f, "'", `'\''`) + "'"
	}
	wrapper := fmt.Sprintf("#!/bin/sh\n# enowX: containers started here send their HTTP through Traffic.\ncase \"$1\" in\nrun|create)\n\tsub=$1\n\tshift\n\texec '%s' \"$sub\" %s \"$@\"\n\t;;\nesac\nexec '%s' \"$@\"\n",
		real, strings.Join(quoted, " "), real)
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(wrapper), 0o755); err != nil {
		return proto.TrafficTarget{}, err
	}
	tab, err := s.createSession(proto.SessionCreateParams{
		Parent: p.Session, Name: "Docker (Traffic)", Agent: agent.Custom,
		Command: []string{"/bin/sh", "-c", `PATH="$0:$PATH" exec "${SHELL:-/bin/sh}"`, dir},
	})
	if err != nil {
		s.detach(tg.Source)
		return proto.TrafficTarget{}, err
	}
	tg.Tab = tab.ID()
	tg.Note = "Containers started with docker run or create from the new tab send their HTTP through Traffic, trusting enowX's CA."
	s.targetsChanged()
	return tg.TrafficTarget, nil
}

// bridgeGateway is the docker0 bridge's address, where containers reach
// their host.
var dockerGateway = bridgeGateway

func bridgeGateway() (string, error) {
	iface, err := net.InterfaceByName("docker0")
	if err != nil {
		return "", errors.New("traffic: no docker0 bridge here: is Docker running?")
	}
	addrs, _ := iface.Addrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
			return ipn.IP.String(), nil
		}
	}
	return "", errors.New("traffic: docker0 has no IPv4 address")
}
