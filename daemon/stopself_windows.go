package daemon

import "os"

// stopSelf ends the daemon; Windows has no polite signal for it.
func stopSelf() { os.Exit(0) }
