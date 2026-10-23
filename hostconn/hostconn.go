// Package hostconn connects the app to the daemon of a host: this machine's,
// started on demand, or a server's over SSH, installed or upgraded on
// demand. Either way the daemon is this build's version when it returns.
package hostconn

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/core/tailnet"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
	"github.com/enowx/enowx/version"
)

//go:embed daemons
var daemons embed.FS

// Options configures a connection.
type Options struct {
	// OnEvent receives the daemon's events (see client.New).
	OnEvent func(proto.Event)
	// Progress, if set, is told what is happening ("Installing enowxd…").
	Progress func(string)
	// Banner sees what a server says while signing in (transport's
	// SSHOptions.Banner): Tailscale SSH's link to check the user in a
	// browser, which the connection waits for.
	Banner func(msg string) error
	// TrustHost is asked about servers missing from known_hosts.
	TrustHost func(addr, fingerprint string) bool
	// DaemonCommand starts this machine's daemon when none runs.
	DaemonCommand []string
	// Password gives the SSH password of hosts with PasswordAuth.
	Password func(hosts.Host) (string, error)
	// Passphrase is asked for the passphrase of an SSH key the server
	// accepts.
	Passphrase func(path string) (string, error)
	// RunAs is asked, when SSH signs in as root to a host that has no
	// choice saved, whether to run enowX as a regular user, given the
	// server's users. It returns the host with RunAs or KeepRoot set, and is
	// expected to save it.
	RunAs func(hosts.Host, []ServerUser) (hosts.Host, error)
}

func (o Options) progress(s string) {
	if o.Progress != nil {
		o.Progress(s)
	}
}

// Connect connects to the daemon of h: this machine's for hosts.LocalID.
func Connect(h hosts.Host, opts Options) (*client.Client, error) {
	if h.ID == hosts.LocalID {
		return connectLocal(opts)
	}
	return connectRemote(h, opts)
}

func connectLocal(opts Options) (*client.Client, error) {
	path, err := transport.SocketPath()
	if err != nil {
		return nil, err
	}
	dial := func() (*client.Client, error) {
		var spawn *exec.Cmd
		if len(opts.DaemonCommand) > 0 {
			spawn = exec.Command(opts.DaemonCommand[0], opts.DaemonCommand[1:]...)
		}
		conn, err := transport.DialLocal(path, spawn)
		if err != nil {
			return nil, err
		}
		return client.New(conn, opts.OnEvent)
	}
	c, err := dial()
	if err != nil {
		return nil, err
	}
	info, err := c.Info()
	if err != nil {
		c.Close()
		return nil, err
	}
	if info.Version == version.String && info.Build == version.Build() {
		return c, nil
	}
	// A daemon left running by another version of the app speaks another
	// protocol, and one from another build of this version runs other code:
	// replace it. Its sessions end with it.
	if info.Version == version.String {
		opts.progress("Restarting the daemon (new build of the app)")
	} else {
		opts.progress("Restarting the daemon (" + info.Version + " → " + version.String + ")")
	}
	c.Close()
	if err := transport.StopDaemon(info.PID, path); err != nil {
		return nil, err
	}
	return dial()
}

func connectRemote(h hosts.Host, opts Options) (*client.Client, error) {
	target, err := transport.ParseSSHTarget(h.Address)
	if err != nil {
		return nil, err
	}
	if h.Jump != "" {
		target.Jump = h.Jump
	}
	target = target.WithSSHConfig()
	opts.progress("Connecting to " + target.String())
	target.Host = resolveTailnet(target.Host)
	sshOpts := transport.SSHOptions{IdentityFile: h.IdentityFile, TrustHost: opts.TrustHost, Passphrase: opts.Passphrase, Banner: opts.Banner}
	if h.PasswordAuth && opts.Password != nil {
		sshOpts.Password = func() (string, error) { return opts.Password(h) }
	}
	ssh, err := transport.DialSSH(target, sshOpts)
	if err != nil {
		return nil, err
	}
	user, err := runAsUser(ssh, h, opts)
	if err != nil {
		ssh.Close()
		return nil, err
	}
	arch, err := ensureDaemon(ssh, user, opts)
	if err != nil {
		ssh.Close()
		return nil, err
	}
	// The daemon running may still be an older one than the file installed
	// (it outlived an install that couldn't stop it): replace it, once. The
	// probe's stream leaves the SSH connection open for the stop.
	probeOpts := opts
	probeOpts.OnEvent = nil // the app hears the daemon it keeps
	probe, err := bridge(ssh, user, target, probeOpts, ssh.BridgeSession)
	if err != nil {
		return nil, err
	}
	info, err := probe.Info()
	probe.Close()
	if err == nil && runsOther(info.Hash, embeddedHash(arch)) {
		opts.progress("Restarting enowxd on " + ssh.Target.Host + " (it runs an older build)")
		if _, err := ssh.Run(transport.AsUser(user, `"$HOME/`+transport.RemoteDaemonPath+`" stop`), nil); err != nil {
			ssh.Close()
			return nil, fmt.Errorf("hostconn: restart enowxd on %s: %w", ssh.Target.Host, err)
		}
	}
	c, err := bridge(ssh, user, target, opts, ssh.Bridge)
	if err != nil {
		return nil, err
	}
	c.SetTunnel(ssh.DialTCP)
	return c, nil
}

// bridge opens a client to the server's daemon, starting it when needed.
func bridge(ssh *transport.SSHClient, user string, target transport.SSHTarget, opts Options, open func(string) (io.ReadWriteCloser, error)) (*client.Client, error) {
	stream, err := open(user)
	if err != nil {
		ssh.Close()
		return nil, err
	}
	c, err := client.New(stream, opts.OnEvent)
	if err != nil {
		stream.Close()
		return nil, fmt.Errorf("hostconn: %s: %w", target, err)
	}
	return c, nil
}

// runsOther reports whether a daemon running the executable hashed have
// runs other code than this build carries (want). A daemon that tells no
// hash predates hashes, so it is older; without want nothing tells.
func runsOther(have, want string) bool { return want != "" && have != want }

// runAsUser returns the user enowX runs as on the server, "" for the SSH
// user. Agents won't skip permission prompts as root, so root logins run the
// daemon, and every session, as a regular user the host names, created here
// when missing.
func runAsUser(ssh *transport.SSHClient, h hosts.Host, opts Options) (string, error) {
	out, err := ssh.Run("id -u", nil)
	if err != nil {
		return "", fmt.Errorf("hostconn: inspect server: %w", err)
	}
	if strings.TrimSpace(out) != "0" {
		return "", nil
	}
	if h.RunAs == "" && !h.KeepRoot && opts.RunAs != nil {
		if h, err = opts.RunAs(h, serverUsers(ssh)); err != nil {
			return "", err
		}
	}
	if h.KeepRoot || h.RunAs == "" {
		return "", nil
	}
	if !hosts.ValidUserName(h.RunAs) {
		return "", fmt.Errorf("hostconn: %q is not a valid user name", h.RunAs)
	}
	opts.progress("Preparing user " + h.RunAs + " on " + ssh.Target.Host)
	create := `id -u ` + h.RunAs + ` >/dev/null 2>&1 || useradd -m -s /bin/bash ` + h.RunAs + ` || adduser -D -s /bin/sh ` + h.RunAs
	if _, err := ssh.Run(create, nil); err != nil {
		return "", fmt.Errorf("hostconn: create user %s on %s: %w", h.RunAs, ssh.Target.Host, err)
	}
	return h.RunAs, nil
}

// resolveTailnet returns the Tailscale IP of a MagicDNS host when this
// process can't resolve its name, and host otherwise.
func resolveTailnet(host string) string {
	if !tailnet.IsMagicDNS(host) {
		return host
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := net.DefaultResolver.LookupHost(ctx, host); err == nil {
		return host
	}
	if ip, err := tailnet.Resolve(ctx, host); err == nil {
		return ip
	}
	return host
}

// ServerUser is a regular user of a server.
type ServerUser struct {
	Name string
	Home string
	// HasDaemon: enowX ran as this user before, from this device or another.
	HasDaemon bool
}

// serverUsers lists the server's people: users with a login shell and a
// uid from 1000, which leaves out root and system accounts. A server that
// can't say lists none.
func serverUsers(ssh *transport.SSHClient) []ServerUser {
	const list = `(getent passwd 2>/dev/null || cat /etc/passwd) | awk -F: '$3 >= 1000 && $3 < 60000 && $7 !~ /(nologin|false)$/ ` +
		`{ print $1 ":" $6 ":" (system("test -x \"" $6 "/` + transport.RemoteDaemonPath + `\"") == 0) }'`
	out, err := ssh.Run(list, nil)
	if err != nil {
		return nil
	}
	return parseServerUsers(out)
}

func parseServerUsers(out string) []ServerUser {
	var users []ServerUser
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(line, ":")
		if len(f) != 3 || !hosts.ValidUserName(f[0]) {
			continue
		}
		users = append(users, ServerUser{Name: f[0], Home: f[1], HasDaemon: f[2] == "1"})
	}
	return users
}

// ensureDaemon installs this build's enowxd for user (see runAsUser) unless
// it is there already, and stops an older daemon so the bridge starts the
// new one.
func ensureDaemon(ssh *transport.SSHClient, user string, opts Options) (string, error) {
	bin := `"$HOME/` + transport.RemoteDaemonPath + `"`
	out, err := ssh.Run(transport.AsUser(user, inspectScript(bin)), nil)
	if err != nil {
		return "", fmt.Errorf("hostconn: inspect server: %w", err)
	}
	uname, have, hash := parseInspect(out)
	arch, err := linuxArch(uname)
	if err != nil {
		return "", err
	}
	if want := embeddedHash(arch); have == version.String && (hash == "" || want == "" || hash == want) {
		return arch, nil
	}
	f, err := daemons.Open("daemons/enowxd-linux-" + arch + ".gz")
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("hostconn: this build of enowX has no Linux daemon for %s; install enowxd %s at ~/%s on the server", arch, version.String, transport.RemoteDaemonPath)
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	if have == version.String {
		opts.progress("Updating enowxd on " + ssh.Target.Host + " (another build of " + version.String + ")")
	} else {
		opts.progress("Installing enowxd " + version.String + " on " + ssh.Target.Host)
	}
	install := `set -e; mkdir -p "$HOME/.enowx/bin"; cat > ` + bin + `.tmp; chmod 755 ` + bin + `.tmp; ` +
		`mv -f ` + bin + `.tmp ` + bin + `; ` + bin + ` stop >/dev/null 2>&1 || true`
	if _, err := ssh.Run(transport.AsUser(user, install), io.Reader(gz)); err != nil {
		return "", fmt.Errorf("hostconn: install enowxd on %s: %w", ssh.Target.Host, err)
	}
	return arch, nil
}

// inspectScript asks the server its system, and the version of the daemon
// at bin and its executable's SHA-256 where the server has a tool for it:
// builds of one version (from source, between releases) differ only there.
func inspectScript(bin string) string {
	return `uname -sm; printf 'v:'; ` + bin + ` version 2>/dev/null; echo; printf 'h:'; ` +
		`{ sha256sum ` + bin + ` 2>/dev/null || shasum -a 256 ` + bin + ` 2>/dev/null; } | cut -d' ' -f1; echo`
}

// parseInspect reads what ensureDaemon asks the server: `uname -sm`, then
// the daemon's version and hash after "v:" and "h:" ("" when missing).
func parseInspect(out string) (uname, version, hash string) {
	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case i == 0:
			uname = line
		case strings.HasPrefix(line, "v:"):
			version = strings.TrimSpace(strings.TrimPrefix(line, "v:"))
		case strings.HasPrefix(line, "h:"):
			hash = strings.TrimSpace(strings.TrimPrefix(line, "h:"))
		}
	}
	return uname, version, hash
}

// embeddedHashes are the SHA-256s of the daemons this build carries, by
// architecture, worked out once.
var embeddedHashes sync.Map

// embeddedHash is the SHA-256 of the daemon this build carries for arch,
// "" when it carries none.
func embeddedHash(arch string) string {
	if h, ok := embeddedHashes.Load(arch); ok {
		return h.(string)
	}
	h := ""
	if f, err := daemons.Open("daemons/enowxd-linux-" + arch + ".gz"); err == nil {
		defer f.Close()
		if gz, err := gzip.NewReader(f); err == nil {
			sum := sha256.New()
			if _, err := io.Copy(sum, gz); err == nil {
				h = hex.EncodeToString(sum.Sum(nil))
			}
		}
	}
	embeddedHashes.Store(arch, h)
	return h
}

// linuxArch maps `uname -sm` to a GOARCH the app carries a daemon for.
func linuxArch(uname string) (string, error) {
	fields := strings.Fields(uname)
	if len(fields) != 2 || fields[0] != "Linux" {
		return "", fmt.Errorf("hostconn: servers must run Linux for now, this one runs %q", uname)
	}
	switch fields[1] {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	}
	return "", fmt.Errorf("hostconn: unsupported server architecture %q", fields[1])
}
