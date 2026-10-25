package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/core/vault"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/vaultview"
)

// vaultOptions are the page's.
func (a *App) vaultOptions() vaultview.Options {
	v := a.vaultOf()
	o := vaultview.Options{
		State: v.state(), Hosts: a.vaultHosts(), Current: vaultview.Host{ID: a.host.ID, Name: a.host.Name},
		Audit: v.audit, Testing: v.testing, Remember: a.state.VaultRemember, Found: v.found, Scanning: v.scan,
	}
	if v.data != nil {
		o.Credentials = v.data.Credentials
	}
	o.Actions = vaultview.Actions{
		Create: a.vaultCreate,
		Unlock: a.vaultUnlock,
		Lock:   a.vaultLock,
		Save: func(c vault.Credential) error {
			if v.data == nil {
				return errors.New("the vault is locked")
			}
			if c.Type == vault.TypeSSHKey && strings.TrimSpace(c.Fields["public_key"]) == "" {
				if pub, err := vault.PublicKey(c.Fields["private_key"]); err == nil {
					c.Fields["public_key"] = pub
				}
			}
			if c.ID == "" {
				if _, err := v.data.Add(c); err != nil {
					return err
				}
			} else if old, ok := v.data.Find(c.ID); ok {
				if other, taken := v.data.Find(c.Name); taken && other.ID != c.ID {
					return fmt.Errorf("a credential is already named %q", c.Name)
				}
				*old = c
			}
			return a.vaultSave()
		},
		Remove: func(id string) {
			if v.data != nil && v.data.Remove(id) == nil {
				_ = a.vaultSave()
			}
		},
		Test: func(id string) { a.vaultTest(id, func(string, string) {}) },
		SetDefault: func(id string) {
			if v.data != nil && v.data.SetDefault(id) == nil {
				_ = a.vaultSave()
			}
		},
		Revoke: func(id string, revoked bool) {
			if c, ok := v.data.Find(id); ok {
				c.Revoked = revoked
				_ = a.vaultSave()
			}
		},
		Scan:   a.vaultScan,
		Import: a.vaultImport,
		ChangePassword: func(old, password string) error {
			if err := v.file.ChangePassword(old, password); err != nil {
				return err
			}
			return a.vaultSave()
		},
	}
	if a.client != nil {
		o.Actions.Login = a.vaultLogin
	}
	o.ClientIDs = a.state.OAuthClients
	o.Actions.DeviceLogin = a.vaultDeviceLogin
	return o
}

// vaultScan finds the logins the connected host's tools keep.
func (a *App) vaultScan() {
	v, cl := a.vaultOf(), a.client
	if cl == nil {
		return
	}
	v.scan, v.found = true, nil
	go func() {
		found, err := cl.VaultScan()
		a.onMain(func() {
			v.scan, v.found = false, found
			if err != nil {
				a.status = err.Error()
			}
		})
	}()
}

// vaultImport keeps the logins picked of those found.
func (a *App) vaultImport(picked []proto.VaultFound) error {
	v := a.vaultOf()
	if v.data == nil {
		return errors.New("the vault is locked")
	}
	for _, f := range picked {
		c := vault.Credential{Platform: f.Platform, Account: f.Account, Type: f.Type, Fields: f.Fields, Name: f.Name, Notes: "imported from " + f.Source}
		for i := 2; ; i++ {
			if _, taken := v.data.Find(c.Name); !taken {
				break
			}
			c.Name = fmt.Sprintf("%s-%d", f.Name, i)
		}
		if _, err := v.data.Add(c); err != nil {
			return err
		}
	}
	v.found = nil
	return a.vaultSave()
}

// vaultLogin runs platform's own login in a new session of the connected
// host, for the user to import afterwards.
func (a *App) vaultLogin(platform string) {
	p, cl := vault.Lookup(platform), a.client
	if p.Login == "" || cl == nil {
		return
	}
	project := a.project
	go func() {
		info, err := cl.CreateSession(proto.SessionCreateParams{Name: "Log in to " + p.Name, Project: project, BranchMode: proto.BranchNone,
			Command: []string{"sh", "-c", p.Login + `; echo; echo "Done: import it in enowX → Credentials."; exec "${SHELL:-sh}"`}, Cols: 100, Rows: 30})
		a.onMain(func() {
			if err != nil {
				a.status = err.Error()
				return
			}
			a.upsert(info)
			a.selectSession(info)
			a.notify("Log in to " + p.Name + " in the new session, then Import in Credentials")
		})
	}()
}

// vaultDeviceLogin signs in to platform in the browser with the user's
// OAuth app clientID, kept for next time; the token goes to the form, for
// the user to add.
func (a *App) vaultDeviceLogin(platform, clientID string, code func(userCode, url string), done func(string, error)) {
	p := vault.Lookup(platform)
	if p.Device == nil {
		done("", fmt.Errorf("%s has no sign-in in the browser", p.Name))
		return
	}
	clientID = strings.TrimSpace(clientID)
	if clientID != "" && a.state.OAuthClients[platform] != clientID {
		if a.state.OAuthClients == nil {
			a.state.OAuthClients = map[string]string{}
		}
		a.state.OAuthClients[platform] = clientID
		a.saveState()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		dc, err := vault.StartDevice(ctx, *p.Device, clientID)
		if err != nil {
			a.onMain(func() { done("", err) })
			return
		}
		a.onMain(func() { code(dc.UserCode, dc.URL) })
		_ = openExternal(dc.URL)
		token, err := dc.Wait(ctx)
		a.onMain(func() { done(token, err) })
	}()
}

// openExternal opens url with the system (the browser); tests replace it.
var openExternal = func(url string) error { return mygo.Shell.OpenExternal(url) }
