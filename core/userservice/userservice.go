// Package userservice runs the server daemon as a systemd user service,
// with lingering, so it starts again after a reboot and automations keep
// running with nobody signed in. Where there is no systemd (Alpine,
// containers), Start reports it and the daemon is started as a process.
package userservice

// Unit is the service's name.
const Unit = "enowxd.service"
