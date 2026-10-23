package version

import (
	"fmt"
	"os"
	"sync"
)

// Build identifies this executable: its file's time and size. Builds of
// one version keep the same String (development rebuilds, a build from
// source past a release), so this is how the app tells that the local
// daemon, which runs the app's own executable, was started by another
// build and runs other code.
var Build = sync.OnceValue(func() string {
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
