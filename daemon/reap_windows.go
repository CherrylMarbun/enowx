package daemon

// tag marks the processes something started (see reap_unix.go).
type tag struct{ key, value string }

// reap and reapAll do nothing on Windows, where a process's environment
// isn't read: a session's processes end with its console.
func reap(tag)      {}
func reapAll([]tag) {}
