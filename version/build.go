package version

import (
	"fmt"
	"os"
	"sync"
)

// Build identifies this executable when ENOWX_DEV=1, and is "" otherwise.
// Development rebuilds keep the same String, so this is how the app tells
// that a daemon started by an earlier build runs old code.
var Build = sync.OnceValue(func() string {
	if os.Getenv("ENOWX_DEV") != "1" {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	info, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x-%x", info.ModTime().UnixNano(), info.Size())
})
