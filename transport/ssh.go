package transport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// RemoteDaemonPath is where enowxd lives on servers, relative to $HOME.
const RemoteDaemonPath = ".enowx/bin/enowxd"

// SSHTarget is where to connect: "user@host", "host:port" or "user@host:port".
type SSHTarget struct {
	User string
	Host string
	Port string
}

// String formats the target as user@host, with :port when it isn't 22.
func (t SSHTarget) String() string {
	s := t.User + "@" + t.Host
	if t.Port != "" && t.Port != "22" {
		s += ":" + t.Port
	}
	return s
}

// ParseSSHTarget parses "[user@]host[:port]".
func ParseSSHTarget(s string) (SSHTarget, error) {
	s = strings.TrimSpace(s)
	t := SSHTarget{Port: "22"}
	if at := strings.LastIndex(s, "@"); at >= 0 {
		t.User, s = s[:at], s[at+1:]
	}
	if host, port, err := net.SplitHostPort(s); err == nil {
		t.Host, t.Port = host, port
	} else {
		t.Host = s
	}
	if t.Host == "" || strings.ContainsAny(t.Host, " /") {
		return SSHTarget{}, fmt.Errorf("transport: %q is not a host", s)
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

// SSHOptions configures DialSSH.
type SSHOptions struct {
	// IdentityFile is a private key to try before the default ones.
	IdentityFile string
	// Password, if set, gives the password to try after the keys, for
	// password and keyboard-interactive logins. It is asked only if needed.
	Password func() (string, error)
	// TrustHost is asked about a host missing from known_hosts, with its key's
	// type and SHA256 fingerprint ("ecdsa-sha2-nistp256 SHA256:…"); when it
	// answers true the key is added. Nil refuses
	// unknown hosts. Hosts whose key changed are always refused.
	TrustHost func(addr, fingerprint string) bool
}

// SSHClient is an SSH connection to a server.
type SSHClient struct {
	client *ssh.Client
	Target SSHTarget
}

// DialSSH connects and authenticates with the SSH agent and the user's
// keys, checking the host key against ~/.ssh/known_hosts.
func DialSSH(target SSHTarget, opts SSHOptions) (*SSHClient, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	knownPath := filepath.Join(home, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(knownPath), 0o700); err != nil {
		return nil, err
	}
	if f, err := os.OpenFile(knownPath, os.O_CREATE|os.O_RDONLY, 0o600); err == nil {
		f.Close()
	}
	known, err := knownhosts.New(knownPath)
	if err != nil {
		return nil, fmt.Errorf("transport: read known_hosts: %w", err)
	}
	addr := net.JoinHostPort(target.Host, target.Port)
	cfg := &ssh.ClientConfig{
		User:            target.User,
		Auth:            authMethods(home, opts.IdentityFile, opts.Password),
		HostKeyCallback: trustOnFirstUse(known, knownPath, opts.TrustHost),
		Timeout:         15 * time.Second,
	}
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		var keyErr *knownhosts.KeyError
		switch {
		case errors.As(err, &keyErr) && len(keyErr.Want) > 0:
			return nil, fmt.Errorf("transport: the host key of %s changed; if that is expected, remove its line from %s", addr, knownPath)
		case errors.As(err, &keyErr):
			return nil, fmt.Errorf("transport: %s is not a trusted host", addr)
		}
		return nil, fmt.Errorf("transport: ssh %s: %w", addr, err)
	}
	return &SSHClient{client: conn, Target: target}, nil
}

// trustOnFirstUse checks known_hosts and asks trust about unknown hosts.
func trustOnFirstUse(known ssh.HostKeyCallback, path string, trust func(addr, fingerprint string) bool) ssh.HostKeyCallback {
	var mu sync.Mutex
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := known(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		if err == nil || !errors.As(err, &keyErr) || len(keyErr.Want) > 0 || trust == nil {
			return err
		}
		if !trust(hostname, key.Type()+" "+ssh.FingerprintSHA256(key)) {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		f, ferr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if ferr != nil {
			return ferr
		}
		defer f.Close()
		_, ferr = fmt.Fprintln(f, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key))
		return ferr
	}
}

// Run runs command on the server with stdin and returns its output.
func (c *SSHClient) Run(command string, stdin io.Reader) (string, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	var out, errOut bytes.Buffer
	sess.Stdin, sess.Stdout, sess.Stderr = stdin, &out, &errOut
	if err := sess.Run(command); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return out.String(), fmt.Errorf("%w: %s", err, msg)
		}
		return out.String(), err
	}
	return out.String(), nil
}

// Bridge runs `enowxd bridge` on the server and returns its stdin/stdout as
// one stream to the daemon. Closing the stream closes the SSH connection.
func (c *SSHClient) Bridge() (io.ReadWriteCloser, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := sess.Start(`"$HOME/` + RemoteDaemonPath + `" bridge`); err != nil {
		return nil, fmt.Errorf("transport: start bridge on %s: %w", c.Target, err)
	}
	return &sshStream{Reader: stdout, Writer: stdin, sess: sess, conn: c.client}, nil
}

// Close closes the connection.
func (c *SSHClient) Close() error { return c.client.Close() }

// authMethods uses the SSH agent when available, then unencrypted keys
// (identityFile first, then the default ones), then the password.
func authMethods(home, identityFile string, password func() (string, error)) []ssh.AuthMethod {
	var methods []ssh.AuthMethod
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if c, err := net.Dial("unix", sock); err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(c).Signers))
		}
	}
	files := []string{}
	if identityFile != "" {
		if strings.HasPrefix(identityFile, "~/") {
			identityFile = filepath.Join(home, identityFile[2:])
		}
		files = append(files, identityFile)
	}
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		files = append(files, filepath.Join(home, ".ssh", name))
	}
	var signers []ssh.Signer
	for _, f := range files {
		pem, err := os.ReadFile(f)
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
	if password != nil {
		methods = append(methods,
			ssh.PasswordCallback(password),
			// Servers that ask for the password as a challenge get it for
			// every question, which is what OpenSSH does with one prompt.
			ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
				pw, err := password()
				if err != nil {
					return nil, err
				}
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = pw
				}
				return answers, nil
			}),
		)
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
