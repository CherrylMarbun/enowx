package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// RemoteBridgeCommand runs on the server to bridge the SSH session's
// stdin/stdout to the daemon's socket, starting the daemon if needed.
const RemoteBridgeCommand = "$HOME/.enowx/bin/enowxd bridge"

// SSHTarget is where to connect: "user@host", "host:port" or "user@host:port".
type SSHTarget struct {
	User string
	Host string
	Port string
}

// ParseSSHTarget parses "[user@]host[:port]".
func ParseSSHTarget(s string) (SSHTarget, error) {
	t := SSHTarget{Port: "22"}
	if at := strings.LastIndex(s, "@"); at >= 0 {
		t.User, s = s[:at], s[at+1:]
	}
	if host, port, err := net.SplitHostPort(s); err == nil {
		t.Host, t.Port = host, port
	} else {
		t.Host = s
	}
	if t.Host == "" {
		return SSHTarget{}, errors.New("transport: empty host")
	}
	if t.User == "" {
		u, err := user.Current()
		if err != nil {
			return SSHTarget{}, err
		}
		t.User = u.Username
	}
	return t, nil
}

// DialSSH connects to target and runs command (RemoteBridgeCommand when
// empty), returning the session's stdin/stdout as one stream. Host keys
// are checked against ~/.ssh/known_hosts; unknown hosts are refused.
func DialSSH(target SSHTarget, command string) (io.ReadWriteCloser, error) {
	if command == "" {
		command = RemoteBridgeCommand
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	hostKeys, err := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("transport: read known_hosts: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User:            target.User,
		Auth:            authMethods(home),
		HostKeyCallback: hostKeys,
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(target.Host, target.Port)
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
			return nil, fmt.Errorf("transport: %s is not in known_hosts; connect once with ssh to verify its key", addr)
		}
		return nil, fmt.Errorf("transport: ssh %s: %w", addr, err)
	}
	sess, err := conn.NewSession()
	if err != nil {
		conn.Close()
		return nil, err
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		conn.Close()
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := sess.Start(command); err != nil {
		conn.Close()
		return nil, fmt.Errorf("transport: run %q on %s: %w", command, addr, err)
	}
	return &sshStream{Reader: stdout, Writer: stdin, sess: sess, conn: conn}, nil
}

// authMethods uses the SSH agent when available, then unencrypted default keys.
func authMethods(home string) []ssh.AuthMethod {
	var methods []ssh.AuthMethod
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if c, err := net.Dial("unix", sock); err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(c).Signers))
		}
	}
	var signers []ssh.Signer
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		pem, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err != nil {
			continue
		}
		if s, err := ssh.ParsePrivateKey(pem); err == nil {
			signers = append(signers, s)
		}
	}
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	return methods
}

type sshStream struct {
	io.Reader
	io.Writer
	sess *ssh.Session
	conn *ssh.Client
}

func (s *sshStream) Close() error {
	_ = s.sess.Close()
	return s.conn.Close()
}
