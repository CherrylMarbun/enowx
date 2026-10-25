//go:build !windows

package daemon

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/enowx/enowx/core/vault"
	"github.com/enowx/enowx/proto"
)

// exeSuffix is a program's file extension.
const exeSuffix = ""

// startSSHAgent serves session id an SSH agent of the vault's keys: it
// lists the public keys of the host's index, and asks the app to sign,
// the private keys never leaving it. The socket lives in a folder of its
// own under the system's temporary folder, as a socket's path is short.
func (s *Server) startSSHAgent(id string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "enowx-ssh-")
	if err != nil {
		return "", nil, err
	}
	sock := filepath.Join(dir, "agent.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	a := &vault.SSHAgent{
		Keys: func() []string {
			var keys []string
			for _, c := range s.vaultIndex().Credentials {
				if c.PublicKey != "" && !c.Revoked {
					keys = append(keys, strings.TrimSpace(c.PublicKey)+" "+c.Name)
				}
			}
			return keys
		},
		SignData: func(key, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
			lend, err := s.vaultAsk(proto.VaultAsk{Action: proto.VaultSign, Session: id, KeyBlob: key, Data: data, Flags: uint32(flags), Command: "ssh"})
			if err != nil {
				return nil, err
			}
			if len(lend.Signature) == 0 {
				return nil, errors.New("vault: no signature")
			}
			return &ssh.Signature{Format: lend.SigFormat, Blob: lend.Signature}, nil
		},
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() { _ = a.Serve(conn) }()
		}
	}()
	return sock, func() {
		_ = l.Close()
		_ = os.RemoveAll(dir)
	}, nil
}
