package vault

import (
	"crypto/rand"
	"errors"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSHAgent is an SSH agent whose keys are the vault's: it lists their
// public keys and has SignData, the app holding the private keys, make each
// signature. A private key is never on the host.
type SSHAgent struct {
	// Keys is the public keys, as in authorized_keys, with a comment.
	Keys func() []string
	// SignData signs data with the key whose wire form is key.
	SignData func(key, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error)
}

var errManaged = errors.New("the vault's keys are kept in the enowX app")

// Serve answers one connection of an SSH client.
func (a *SSHAgent) Serve(conn net.Conn) error {
	defer conn.Close()
	return agent.ServeAgent(a, conn)
}

// List implements agent.Agent.
func (a *SSHAgent) List() ([]*agent.Key, error) {
	var out []*agent.Key
	for _, line := range a.Keys() {
		pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			continue
		}
		out = append(out, &agent.Key{Format: pub.Type(), Blob: pub.Marshal(), Comment: strings.TrimSpace(comment)})
	}
	return out, nil
}

// Sign implements agent.Agent.
func (a *SSHAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.SignWithFlags(key, data, 0)
}

// SignWithFlags implements agent.ExtendedAgent.
func (a *SSHAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return a.SignData(key.Marshal(), data, flags)
}

// Add implements agent.Agent.
func (a *SSHAgent) Add(agent.AddedKey) error { return errManaged }

// Remove implements agent.Agent.
func (a *SSHAgent) Remove(ssh.PublicKey) error { return errManaged }

// RemoveAll implements agent.Agent.
func (a *SSHAgent) RemoveAll() error { return errManaged }

// Lock implements agent.Agent.
func (a *SSHAgent) Lock([]byte) error { return errManaged }

// Unlock implements agent.Agent.
func (a *SSHAgent) Unlock([]byte) error { return errManaged }

// Signers implements agent.Agent.
func (a *SSHAgent) Signers() ([]ssh.Signer, error) { return nil, errManaged }

// Extension implements agent.ExtendedAgent.
func (a *SSHAgent) Extension(string, []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}

// SignSSH signs data with the private key, as an agent asked: flags pick
// SHA-2 for an RSA key.
func SignSSH(private string, data []byte, flags uint32) (*ssh.Signature, error) {
	signer, err := ssh.ParsePrivateKey([]byte(private))
	if err != nil {
		return nil, err
	}
	if as, ok := signer.(ssh.AlgorithmSigner); ok && signer.PublicKey().Type() == ssh.KeyAlgoRSA {
		switch {
		case flags&uint32(agent.SignatureFlagRsaSha512) != 0:
			return as.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA512)
		case flags&uint32(agent.SignatureFlagRsaSha256) != 0:
			return as.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA256)
		}
	}
	return signer.Sign(rand.Reader, data)
}

// KeyMatches reports whether the private key is the one whose public key
// has wire form blob.
func KeyMatches(private string, blob []byte) bool {
	signer, err := ssh.ParsePrivateKey([]byte(private))
	return err == nil && string(signer.PublicKey().Marshal()) == string(blob)
}
