package transport

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// A server that takes certificates only, as InstaBox's gateway does: the
// client signs in with a certificate for its key, which it asks for, and
// trusts the server by the authority its host key's certificate is signed
// by; a host signed by another authority is refused.
func TestSignsInWithACertificate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, userKey, _ := ed25519.GenerateKey(rand.Reader)
	block, _ := ssh.MarshalPrivateKey(userKey, "")
	os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), pem.EncodeToMemory(block), 0o600)

	newSigner := func() ssh.Signer {
		_, k, _ := ed25519.GenerateKey(rand.Reader)
		s, _ := ssh.NewSignerFromKey(k)
		return s
	}
	userCA, hostCA, otherCA := newSigner(), newSigner(), newSigner()
	sign := func(ca ssh.Signer, key ssh.PublicKey, kind uint32, principal string) *ssh.Certificate {
		c := &ssh.Certificate{Key: key, CertType: kind, ValidPrincipals: []string{principal},
			ValidAfter: uint64(time.Now().Add(-time.Minute).Unix()), ValidBefore: uint64(time.Now().Add(time.Hour).Unix())}
		if err := c.SignCert(rand.Reader, ca); err != nil {
			t.Fatal(err)
		}
		return c
	}

	hostKey := newSigner()
	hostSigner, _ := ssh.NewCertSigner(sign(hostCA, hostKey.PublicKey(), ssh.HostCert, "127.0.0.1"), hostKey)
	checker := &ssh.CertChecker{IsUserAuthority: func(auth ssh.PublicKey) bool { return string(auth.Marshal()) == string(userCA.PublicKey().Marshal()) }}
	cfg := &ssh.ServerConfig{PublicKeyCallback: checker.Authenticate}
	cfg.AddHostKey(hostSigner)
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			nc, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				if conn, chans, reqs, err := ssh.NewServerConn(nc, cfg); err == nil {
					go ssh.DiscardRequests(reqs)
					go func() {
						for ch := range chans {
							ch.Reject(ssh.Prohibited, "no")
						}
					}()
					conn.Wait()
				}
			}()
		}
	}()
	host, port, _ := net.SplitHostPort(l.Addr().String())
	target := SSHTarget{User: "m_box", Host: host, Port: port}

	asked := 0
	certFor := func(authority ssh.Signer) func(ssh.PublicKey) (string, string, error) {
		return func(key ssh.PublicKey) (string, string, error) {
			asked++
			cert := sign(userCA, key, ssh.UserCert, "m_box")
			return string(ssh.MarshalAuthorizedKey(cert)), string(ssh.MarshalAuthorizedKey(authority.PublicKey())), nil
		}
	}
	c, err := DialSSH(target, SSHOptions{Certificate: certFor(hostCA)})
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if asked != 1 {
		t.Fatalf("certificates asked for: %d", asked)
	}
	if _, err := DialSSH(target, SSHOptions{Certificate: certFor(otherCA)}); err == nil || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("a host signed by another authority: %v", err)
	}
	// known_hosts is left as it was: the authority, not a key, is trusted.
	if data, _ := os.ReadFile(filepath.Join(home, ".ssh", "known_hosts")); len(data) > 0 {
		t.Fatalf("known_hosts: %s", data)
	}
}
