package proto

// MethodSessionPorts lists the TCP ports a session's processes listen on:
// dev servers an agent started, say.
const MethodSessionPorts = "session.ports" // SessionRef → SessionPorts

// SessionPorts are a session's listening ports. Supported is false where
// the daemon can't tell (Windows).
type SessionPorts struct {
	Supported bool   `json:"supported"`
	Ports     []Port `json:"ports,omitempty"`
}

// Port is a TCP port a session's process listens on.
type Port struct {
	Port    int    `json:"port"`
	Addr    string `json:"addr"` // "127.0.0.1", "::1", "*" for every address
	PID     int    `json:"pid"`
	Process string `json:"process"` // the program's name
}
