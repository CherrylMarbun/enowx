package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/core/vault"
	"github.com/enowx/enowx/internal/quietcmd"
	"github.com/enowx/enowx/proto"
)

var vaultHandlers = map[string]handler{
	proto.MethodVaultIndex: bare(func(c *conn) (any, error) { return c.srv.vaultIndex(), nil }),
	proto.MethodVaultGet:   bare(func(c *conn) (any, error) { return c.srv.vaultGet() }),
	proto.MethodVaultPut:   with(func(c *conn, p proto.VaultCopy) (any, error) { return c.srv.vaultPut(p) }),
	proto.MethodVaultServe: bare(func(c *conn) (any, error) {
		c.srv.vault.serve(c)
		return ok, nil
	}),
	proto.MethodVaultReply: with(func(c *conn, p proto.VaultReply) (any, error) {
		c.srv.vault.reply(p)
		return ok, nil
	}),
	proto.MethodVaultAsk: with(func(c *conn, p proto.VaultAsk) (any, error) { return c.srv.vaultAskFiles(p) }),
	proto.MethodVaultAudit: with(func(c *conn, p proto.VaultAuditParams) (any, error) {
		if c.srv.vault.audit == nil {
			return []proto.VaultAuditEntry{}, nil
		}
		return c.srv.vault.audit.List(p.Limit), nil
	}),
	proto.MethodVaultScan: bare(func(c *conn) (any, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		found := vault.Scan(home, runQuiet)
		if found == nil {
			found = []proto.VaultFound{}
		}
		return found, nil
	}),
	proto.MethodVaultAttach: with(func(c *conn, p proto.VaultAttach) (any, error) { return c.srv.vaultAttach(p) }),
}

// vaultAskFiles is vaultAsk with the files a credential lends written for
// the asker: Files and Env then hold their paths, in a folder the asker
// removes when done. What was lent is masked in the session's Traffic.
func (s *Server) vaultAskFiles(a proto.VaultAsk) (proto.VaultLend, error) {
	lend, err := s.vaultAsk(a)
	if err != nil {
		return lend, err
	}
	if l := s.vaultLease(a.Session); l != nil {
		s.vault.mu.Lock()
		for _, v := range lend.Env {
			l.secrets = append(l.secrets, v)
		}
		if lend.Secret != "" {
			l.secrets = append(l.secrets, lend.Secret)
		}
		s.vault.mu.Unlock()
	}
	if len(lend.Files) == 0 {
		return lend, nil
	}
	dir, err := os.MkdirTemp(ensureDir(s.vaultRunDir()), "use-")
	if err != nil {
		return proto.VaultLend{}, err
	}
	if lend.Env == nil {
		lend.Env = map[string]string{}
	}
	for k, content := range lend.Files {
		path := filepath.Join(dir, k)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			_ = os.RemoveAll(dir)
			return proto.VaultLend{}, err
		}
		lend.Env[k], lend.Files[k] = path, path
	}
	return lend, nil
}

func ensureDir(dir string) string {
	_ = os.MkdirAll(dir, 0o700)
	return dir
}

// runQuiet runs a tool for vault.scan (gh auth token), briefly.
func runQuiet(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := quietcmd.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), err
}
