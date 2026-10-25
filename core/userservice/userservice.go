// Package userservice runs the server daemon as a systemd user service,
// with lingering, so it starts again after a reboot and automations keep
// running with nobody signed in. Where there is no systemd (Alpine,
// containers), Start reports it and the daemon is started as a process.
package userservice

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

// Unit is the service's name.
const Unit = "enowxd.service"

// UnitFor is the service of the daemon keeping its state in enowxHome
// (ENOWX_HOME, "" for the user's own, ~/.enowx-app under home): the
// user's daemon is Unit; another's (a test's, a second one) has a unit of
// its own, so starting it never takes the user's daemon's place.
func UnitFor(enowxHome, home string) string {
	if enowxHome == "" || filepath.Clean(enowxHome) == filepath.Join(home, ".enowx-app") {
		return Unit
	}
	sum := sha256.Sum256([]byte(filepath.Clean(enowxHome)))
	return "enowxd-" + hex.EncodeToString(sum[:4]) + ".service"
}
