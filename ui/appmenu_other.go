//go:build !darwin

package ui

// setAppMenu does nothing: only macOS has an app menu; the command palette
// offers "Quit and stop all agents" everywhere.
func setAppMenu(*App) {}
