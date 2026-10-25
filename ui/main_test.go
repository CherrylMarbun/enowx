package ui

import (
	"os"
	"testing"

	"github.com/enowx/enowx/internal/testlisten"
)

func TestMain(m *testing.M) {
	testlisten.Serve()
	// Never the user's enowX: its folder, socket and daemon are a
	// temporary one's (a view connecting to this device's daemon would
	// otherwise replace the user's with the test binary).
	home, err := os.MkdirTemp("", "enowx-ui-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("ENOWX_HOME", home)
	os.Unsetenv("ENOWX_SESSION")
	code := m.Run()
	os.RemoveAll(home)
	// The screen is asked through the window's loop, which tests don't run.
	screenScale = func() int { return 1 }
	os.Exit(code)
}
