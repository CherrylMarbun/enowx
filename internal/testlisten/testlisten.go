// Package testlisten lets a test binary stand in for a dev server that a
// session starts, so tests don't depend on which nc a host has (Debian's
// traditional one ignores the address and port given).
package testlisten

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

const env = "ENOWX_TEST_LISTEN"

// Shell returns a shell command that serves "hi" over HTTP on
// 127.0.0.1:port, as a process named Name.
func Shell(port int) string {
	exe, _ := os.Executable()
	return fmt.Sprintf("%s=%d exec '%s'", env, port, exe)
}

// Name is the name of the process Shell starts.
func Name() string {
	exe, _ := os.Executable()
	return filepath.Base(exe)
}

// Serve serves and never returns when this process was started by Shell;
// tests' TestMain calls it first.
func Serve() {
	port := os.Getenv(env)
	if port == "" {
		return
	}
	if _, err := strconv.Atoi(port); err != nil {
		os.Exit(2)
	}
	_ = http.ListenAndServe("127.0.0.1:"+port, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hi"))
	}))
	os.Exit(1)
}
