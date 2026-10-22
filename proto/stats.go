package proto

// MethodDaemonStats measures what the daemon and its sessions use, and the
// host's memory and CPU: → DaemonStats.
const MethodDaemonStats = "daemon.stats"

// Usage is what a process, or a process tree, uses.
type Usage struct {
	RSS   uint64  `json:"rss"` // bytes of memory
	CPU   float64 `json:"cpu"` // percent of one core
	Procs int     `json:"procs"`
}

// HostUsage is a machine's memory and CPU.
type HostUsage struct {
	CPUs     int     `json:"cpus"`
	MemTotal uint64  `json:"memTotal"`
	MemUsed  uint64  `json:"memUsed"`
	CPU      float64 `json:"cpu"` // percent of all cores
}

// DaemonStats is a measurement of a host.
type DaemonStats struct {
	Supported bool           `json:"supported"` // false where enowX can't measure
	Host      HostUsage      `json:"host"`
	Daemon    Usage          `json:"daemon"`   // the daemon alone
	Sessions  []SessionUsage `json:"sessions"` // each with all it started
}

// SessionUsage is what a session's processes use.
type SessionUsage struct {
	ID string `json:"id"`
	Usage
}
