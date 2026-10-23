package proto

// What a host's daemon keeps for every session it starts, and what it
// says about itself for diagnostics.
const (
	// MethodSessionEnvList lists the variables added to every session's
	// environment, their values masked; MethodSessionEnvSet sets or (with
	// an empty Value) removes one.
	MethodSessionEnvList = "sessionEnv.list" // → []SessionEnvVar
	MethodSessionEnvSet  = "sessionEnv.set"  // SessionEnvVar → []SessionEnvVar
	// MethodDaemonLog reads the end of the daemon's log.
	MethodDaemonLog = "daemon.log" // DaemonLogParams → DaemonLog
	// MethodDaemonRestart stops the daemon after answering; the next
	// connection starts it again, its sessions continued.
	MethodDaemonRestart = "daemon.restart"
)

// SessionEnvVar is a variable of every session's environment. A listed
// one's Value is masked: its last characters only.
type SessionEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// DaemonLogParams asks for the log's last Lines lines.
type DaemonLogParams struct {
	Lines int `json:"lines"`
}

// DaemonLog is the end of the daemon's log.
type DaemonLog struct {
	Path string `json:"path"`
	Text string `json:"text"`
}
