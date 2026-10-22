// Package hostconn connects the app to the daemon of a host: this machine's,
// started on demand, or a server's over SSH, installed or upgraded on
// demand. Either way the daemon is this build's version when it returns.
package hostconn

import (
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
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
	// TrustHost is asked about servers missing from known_hosts.
	TrustHost func(addr, fingerprint string) bool
	// DaemonCommand starts this machine's daemon when none runs.
	DaemonCommand []string
	// Password gives the SSH password of hosts with PasswordAuth.
	Password func(hosts.Host) (string, error)
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
	if info.Version == version.String {
		return c, nil
	}
	// A daemon left running by another version of the app speaks another
	// protocol: replace it. Its sessions end with it.
	opts.progress("Restarting the daemon (" + info.Version + " → " + version.String + ")")
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
	opts.progress("Connecting to " + target.String())
	sshOpts := transport.SSHOptions{IdentityFile: h.IdentityFile, TrustHost: opts.TrustHost}
	if h.PasswordAuth && opts.Password != nil {
		sshOpts.Password = func() (string, error) { return opts.Password(h) }
	}
	ssh, err := transport.DialSSH(target, sshOpts)
	if err != nil {
		return nil, err
	}
	if err := ensureDaemon(ssh, opts); err != nil {
		ssh.Close()
		return nil, err
	}
	stream, err := ssh.Bridge()
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

// ensureDaemon installs this build's enowxd on the server unless it is
// there already, and stops an older daemon so the bridge starts the new one.
func ensureDaemon(ssh *transport.SSHClient, opts Options) error {
	bin := `"$HOME/` + transport.RemoteDaemonPath + `"`
	out, err := ssh.Run(`uname -sm; `+bin+` version 2>/dev/null || true`, nil)
	if err != nil {
		return fmt.Errorf("hostconn: inspect server: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 1 && strings.TrimSpace(lines[1]) == version.String {
		return nil
	}
	arch, err := linuxArch(lines[0])
	if err != nil {
		return err
	}
	f, err := daemons.Open("daemons/enowxd-linux-" + arch + ".gz")
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("hostconn: this build of enowX has no Linux daemon for %s; install enowxd %s at ~/%s on the server", arch, version.String, transport.RemoteDaemonPath)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	opts.progress("Installing enowxd " + version.String + " on " + ssh.Target.Host)
	install := `set -e; mkdir -p "$HOME/.enowx/bin"; cat > ` + bin + `.tmp; chmod 755 ` + bin + `.tmp; ` +
		`mv -f ` + bin + `.tmp ` + bin + `; ` + bin + ` stop >/dev/null 2>&1 || true`
	if _, err := ssh.Run(install, io.Reader(gz)); err != nil {
		return fmt.Errorf("hostconn: install enowxd on %s: %w", ssh.Target.Host, err)
	}
	return nil
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
