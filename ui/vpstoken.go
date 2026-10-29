package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/core/vault"
	"github.com/enowx/enowx/core/vps"
)

// providerToken is the API token of the provider host h was created at:
// the vault's credential for it, the default one, when the vault is open,
// else the one kept in the keychain.
func (a *App) providerToken(h hosts.Host) (string, error) {
	out := make(chan string, 1)
	a.onMain(func() { out <- a.vaultToken(h.Provider) })
	if tok := <-out; tok != "" {
		return tok, nil
	}
	tok, err := mygo.Secrets.Get(tokenKey(h.Provider))
	if err != nil || len(tok) == 0 {
		p, _ := vps.Find(h.Provider)
		return "", fmt.Errorf("no %s API key in the vault or the keychain: add one in the vault to connect to %s", p.Name, h.Name)
	}
	return string(tok), nil
}

// vaultToken is the open vault's API key or token of a provider whose
// platform it has, "" for none. On the main thread.
func (a *App) vaultToken(provider string) string {
	v := a.vaultOf().data
	if v == nil {
		return ""
	}
	for _, c := range v.Pick("", provider, "", func(c *vault.Credential) bool { return !c.Revoked }) {
		for _, k := range []string{"api_key", "token"} {
			if s := strings.TrimSpace(c.Fields[k]); s != "" {
				return s
			}
		}
	}
	return ""
}
