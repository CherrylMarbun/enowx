package ui

import (
	"os"
	"testing"

	"github.com/enowx/enowx/internal/testlisten"
)

func TestMain(m *testing.M) {
	testlisten.Serve()
	// The screen is asked through the window's loop, which tests don't run.
	screenScale = func() int { return 1 }
	os.Exit(m.Run())
}
