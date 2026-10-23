package session

import "strings"

// markerPrefix starts an argument that names a session's process: the
// session's program runs under a small shell given "enowx-session-<id>" as
// its name, so the session's processes can be told apart by their
// arguments, which every platform shows, where their environment
// (EnvSession) can't be read, as on recent macOS.
const markerPrefix = "enowx-session-"

func marker(id string) string { return markerPrefix + id }

// IDFromArgs returns the session a process's arguments name, or "".
func IDFromArgs(args []string) string {
	for _, a := range args {
		if id, ok := strings.CutPrefix(a, markerPrefix); ok && id != "" {
			return id
		}
	}
	return ""
}
