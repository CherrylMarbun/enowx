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
	"slices"
	"strings"
	"sync"
	"time"

	sshconfig "github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// RemoteDaemonPath is where enowxd lives on servers, relative to $HOME.
const RemoteDaemonPath = ".enowx/bin/enowxd"

// SSHTarget is where to connect: "user@host", "host:port" or "user@host:port".
type SSHTarget struct {
	User string // "" until WithSSHConfig: ~/.ssh/config's, else this user's
	Host string
	Port string // "" until WithSSHConfig: ~/.ssh/config's, else 22
	// IdentityFiles are the keys ~/.ssh/config names for the host.
	IdentityFiles []string
}

// String formats the target as [user@]host, with :port when it isn't 22.
func (t SSHTarget) String() string {
	s := t.Host
	if t.Port != "" && t.Port != "22" {
		s = net.JoinHostPort(t.Host, t.Port)
	}
	if t.User != "" {
		s = t.User + "@" + s
	}
	return s
}

// ParseSSHTarget parses "[user@]host[:port]".
func ParseSSHTarget(s string) (SSHTarget, error) {
	s = strings.TrimSpace(s)
	var t SSHTarget
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
	return t, nil
}

// WithSSHConfig fills what the target leaves out from ~/.ssh/config, as
// ssh does: the real host name of an alias, the user, the port and the
// keys. What is still missing gets ssh's defaults.
func (t SSHTarget) WithSSHConfig() SSHTarget {
	cfg := readSSHConfig()
	alias := t.Host
	if h := cfg.get(alias, "HostName"); h != "" {
		t.Host = strings.ReplaceAll(h, "%h", alias)
	}
	if t.User == "" {
		t.User = cfg.get(alias, "User")
	}
	if t.User == "" {
		if u, err := user.Current(); err == nil {
			t.User = u.Username
		}
	}
	if t.Port == "" {
		t.Port = cfg.get(alias, "Port")
	}
	if t.Port == "" {
		t.Port = "22"
	}
	t.IdentityFiles = cfg.getAll(alias, "IdentityFile")
	return t
}

// sshConfig is ~/.ssh/config; a missing or broken file sets nothing.
type sshConfig struct{ cfg *sshconfig.Config }

func readSSHConfig() sshConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return sshConfig{}
	}
	data, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		return sshConfig{}
	}
	cfg, err := sshconfig.DecodeBytes(data)
	if err != nil {
		return sshConfig{}
	}
	return sshConfig{cfg}
}

func (c sshConfig) get(alias, key string) string {
	if c.cfg == nil {
		return ""
	}
	v, _ := c.cfg.Get(alias, key)
	return v
}

func (c sshConfig) getAll(alias, key string) []string {
	if c.cfg == nil {
		return nil
	}
	v, _ := c.cfg.GetAll(alias, key)
	return v
}

// SSHOptions configures DialSSH.
type SSHOptions struct {
	// IdentityFile is a private key to try before the default ones.
	IdentityFile string
	// Password, if set, gives the password to try after the keys, for
	// password and keyboard-interactive logins. It is asked only if needed.
	Password func() (string, error)
	// Passphrase is asked for the passphrase of a key file the server
	// accepts. Nil skips keys with a passphrase.
	Passphrase func(path string) (string, error)
	// TrustHost is asked about a host missing from known_hosts, with its key's
	// type and SHA256 fingerprint ("ecdsa-sha2-nistp256 SHA256:…"); when it
	// answers true the key is added. Nil refuses
	// unknown hosts. Hosts whose key changed are always refused.
	TrustHost func(addr, fingerprint string) bool
}

// AuthError means the server refused every way of signing in offered.
type AuthError struct {
	Addr          string
	User          string
	Allowed       []string // the methods the server allows, as it said
	TriedPassword bool
}

func (e *AuthError) Error() string {
	how := "your SSH keys"
	if e.TriedPassword {
		how += " or the password"
	}
	msg := fmt.Sprintf("transport: %s refused to sign in as %q with %s.", e.Addr, e.User, how)
	if !e.AcceptsPassword() {
		return msg + " It accepts SSH keys only: add your public key to that user's ~/.ssh/authorized_keys"
	}
	return msg + " Check the user name and password"
}

// AcceptsPassword reports whether the server lets users sign in with a
// password, as far as it said.
func (e *AuthError) AcceptsPassword() bool {
	return len(e.Allowed) == 0 || slices.Contains(e.Allowed, "password") || slices.Contains(e.Allowed, "keyboard-interactive")
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
	var offered ssh.PublicKey
	check := trustOnFirstUse(known, knownPath, opts.TrustHost)
	cfg := &ssh.ClientConfig{
		User: target.User,
		Auth: authMethods(home, target, opts),
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			offered = key
			return check(hostname, remote, key)
		},
		// A server has a key of each type; ask for one known_hosts has, or
		// a known server looks like one whose key changed.
		HostKeyAlgorithms: knownAlgorithms(known, addr),
		Timeout:           15 * time.Second,
	}
	// The methods the server allows say whether a password could work.
	var allowed []string
	cfg.AuthCallback = func(ctx *ssh.ClientAuthContext) (ssh.AuthMethod, error) {
		allowed = ctx.AllowedMethods
		return nil, nil
	}
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		var keyErr *knownhosts.KeyError
		switch {
		case errors.As(err, &keyErr) && len(keyErr.Want) > 0:
			got := ""
			if offered != nil {
				got = " (it now shows " + offered.Type() + " " + ssh.FingerprintSHA256(offered) + ")"
			}
			return nil, fmt.Errorf("transport: the host key of %s changed%s; if that is expected, remove its line from %s", addr, got, knownPath)
		case errors.As(err, &keyErr):
			return nil, fmt.Errorf("transport: %s is not a trusted host", addr)
		}
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, &AuthError{Addr: addr, User: target.User, Allowed: allowed, TriedPassword: opts.Password != nil}
		}
		return nil, fmt.Errorf("transport: ssh %s: %w", addr, err)
	}
	return &SSHClient{client: conn, Target: target}, nil
}

// knownAlgorithms returns the host key algorithms of the keys known_hosts
// has for addr, nil for an unknown host.
func knownAlgorithms(known ssh.HostKeyCallback, addr string) []string {
	var keyErr *knownhosts.KeyError
	if err := known(addr, &net.TCPAddr{}, placeholderKey{}); !errors.As(err, &keyErr) {
		return nil
	}
	var algos []string
	for _, k := range keyErr.Want {
		switch t := k.Key.Type(); t {
		case ssh.KeyAlgoRSA:
			algos = append(algos, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA)
		default:
			algos = append(algos, t)
		}
	}
	slices.Sort(algos)
	return slices.Compact(algos)
}

// placeholderKey matches no known key, so known_hosts lists the real ones.
type placeholderKey struct{}

func (placeholderKey) Type() string                        { return "enowx-placeholder" }
func (placeholderKey) Marshal() []byte                     { return []byte("enowx-placeholder") }
func (placeholderKey) Verify([]byte, *ssh.Signature) error { return errors.New("placeholder key") }

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

// AsUser returns command run through `su` as user, so a root login can run
// it as a regular user. An empty user returns command unchanged.
func AsUser(user, command string) string {
	if user == "" {
		return command
	}
	return "su - " + user + " -c " + shellQuote(command)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Bridge runs `enowxd bridge` on the server, as user when set (see AsUser),
// and returns its stdin/stdout as one stream to the daemon. Closing the
// stream closes the SSH connection.
func (c *SSHClient) Bridge(user string) (io.ReadWriteCloser, error) {
	return c.bridge(user, c.client)
}

// BridgeSession is Bridge for a stream that closes only itself, keeping
// the SSH connection for what follows.
func (c *SSHClient) BridgeSession(user string) (io.ReadWriteCloser, error) {
	return c.bridge(user, nil)
}

func (c *SSHClient) bridge(user string, owner *ssh.Client) (io.ReadWriteCloser, error) {
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
	if err := sess.Start(AsUser(user, `"$HOME/`+RemoteDaemonPath+`" bridge`)); err != nil {
		return nil, fmt.Errorf("transport: start bridge on %s: %w", c.Target, err)
	}
	return &sshStream{Reader: stdout, Writer: stdin, sess: sess, conn: owner}, nil
}

// Close closes the connection.
func (c *SSHClient) Close() error { return c.client.Close() }

// DialTCP connects to addr as the server sees it, through the SSH
// connection: "127.0.0.1:3000" is a port on the server.
func (c *SSHClient) DialTCP(addr string) (net.Conn, error) {
	conn, err := c.client.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("transport: reach %s on %s: %w", addr, c.Target.Host, err)
	}
	return conn, nil
}

// authMethods offers the user's keys, then the password. Go's client tries
// the publickey method once, so every key goes into that one method: the
// keys named for the host, the agent's, then the default files.
func authMethods(home string, target SSHTarget, opts SSHOptions) []ssh.AuthMethod {
	methods := []ssh.AuthMethod{ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		return keySigners(home, target, opts), nil
	})}
	if password := opts.Password; password != nil {
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

func keySigners(home string, target SSHTarget, opts SSHOptions) []ssh.Signer {
	expand := func(f string) string {
		if strings.HasPrefix(f, "~/") {
			return filepath.Join(home, f[2:])
		}
		return f
	}
	var named []string
	if opts.IdentityFile != "" {
		named = append(named, expand(opts.IdentityFile))
	}
	for _, f := range target.IdentityFiles {
		named = append(named, expand(f))
	}
	var defaults []string
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		defaults = append(defaults, filepath.Join(home, ".ssh", name))
	}

	var signers []ssh.Signer
	seen := map[string]bool{}
	add := func(s ssh.Signer) {
		if k := string(s.PublicKey().Marshal()); !seen[k] {
			seen[k] = true
			signers = append(signers, s)
		}
	}
	addFiles := func(files []string) {
		for _, f := range files {
			if s := fileSigner(f, opts.Passphrase); s != nil {
				add(s)
			}
		}
	}
	addFiles(named)
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if c, err := net.Dial("unix", sock); err == nil {
			if list, err := agent.NewClient(c).Signers(); err == nil {
				for _, s := range list {
					add(s)
				}
			}
		}
	}
	addFiles(defaults)
	return signers
}

// fileSigner reads a private key. A key with a passphrase asks for it only
// when the server accepts the key, so keys the server never wanted don't
// prompt.
func fileSigner(path string, passphrase func(path string) (string, error)) ssh.Signer {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	s, err := ssh.ParsePrivateKey(pem)
	if err == nil {
		return s
	}
	var missing *ssh.PassphraseMissingError
	if !errors.As(err, &missing) || passphrase == nil {
		return nil
	}
	pub := missing.PublicKey
	if pub == nil {
		// Older key formats hide the public key; the .pub file has it.
		data, err := os.ReadFile(path + ".pub")
		if err != nil {
			return nil
		}
		if pub, _, _, _, err = ssh.ParseAuthorizedKey(data); err != nil {
			return nil
		}
	}
	return &lockedSigner{pub: pub, path: path, pem: pem, passphrase: passphrase}
}

// lockedSigner is a key with a passphrase, unlocked on first use.
type lockedSigner struct {
	pub        ssh.PublicKey
	path       string
	pem        []byte
	passphrase func(path string) (string, error)

	once     sync.Once
	unlocked ssh.AlgorithmSigner
	err      error
}

func (l *lockedSigner) PublicKey() ssh.PublicKey { return l.pub }

func (l *lockedSigner) Sign(r io.Reader, data []byte) (*ssh.Signature, error) {
	return l.SignWithAlgorithm(r, data, "")
}

func (l *lockedSigner) SignWithAlgorithm(r io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	l.once.Do(func() {
		pass, err := l.passphrase(l.path)
		if err != nil {
			l.err = err
			return
		}
		s, err := ssh.ParsePrivateKeyWithPassphrase(l.pem, []byte(pass))
		if err != nil {
			l.err = fmt.Errorf("transport: unlock %s: %w", l.path, err)
			return
		}
		as, ok := s.(ssh.AlgorithmSigner)
		if !ok {
			l.err = fmt.Errorf("transport: %s: unsupported key", l.path)
			return
		}
		l.unlocked = as
	})
	if l.err != nil {
		return nil, l.err
	}
	return l.unlocked.SignWithAlgorithm(r, data, algorithm)
}

type sshStream struct {
	io.Reader
	io.Writer
	sess *ssh.Session
	conn *ssh.Client // closed with the stream, when set
}

func (s *sshStream) Close() error {
	err := s.sess.Close()
	if s.conn != nil {
		err = s.conn.Close()
	}
	return err
}
