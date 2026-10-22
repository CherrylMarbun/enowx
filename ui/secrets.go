package ui

import (
	"errors"
	"fmt"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/core/hosts"
)

// savePassword keeps a host's SSH password in the system's credential
// store. Where there is none (Linux without a keyring), it is kept in
// memory until the app quits, and the user is told.
func (a *App) savePassword(h hosts.Host, password string) {
	err := mygo.Secrets.Set(hosts.PasswordKey(h.ID), []byte(password))
	if err == nil {
		return
	}
	if a.passwords == nil {
		a.passwords = map[string]string{}
	}
	a.passwords[h.ID] = password
	if errors.Is(err, errors.ErrUnsupported) {
		a.status = "No keychain on this system: the password is kept until enowX quits."
	} else {
		a.status = "Couldn't save the password in the keychain: " + err.Error()
	}
}

// password returns a host's SSH password. It may block while the system
// asks the user to unlock the keychain, so it runs off the main thread.
func (a *App) password(h hosts.Host) (string, error) {
	if pw, ok := a.passwordsSnapshot()[h.ID]; ok {
		return pw, nil
	}
	pw, err := mygo.Secrets.Get(hosts.PasswordKey(h.ID))
	if err != nil {
		return "", fmt.Errorf("no saved password for %s; remove the host and add it again", h.Name)
	}
	return string(pw), nil
}

// passwordsSnapshot copies the in-memory passwords on the main thread.
func (a *App) passwordsSnapshot() map[string]string {
	out := make(chan map[string]string, 1)
	a.win.Update(func() {
		m := make(map[string]string, len(a.passwords))
		for k, v := range a.passwords {
			m[k] = v
		}
		out <- m
	})
	return <-out
}

func (a *App) forgetPassword(h hosts.Host) {
	delete(a.passwords, h.ID)
	if h.PasswordAuth {
		_ = mygo.Secrets.Delete(hosts.PasswordKey(h.ID))
	}
}
