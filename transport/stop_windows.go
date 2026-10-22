package transport

import "os"

// stopProcess ends the process; Windows has no polite signal for it.
func stopProcess(p *os.Process) error { return p.Kill() }
