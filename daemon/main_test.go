package daemon

import (
	"os"
	"testing"

	"github.com/enowx/enowx/internal/testlisten"
)

func TestMain(m *testing.M) {
	testlisten.Serve()
	os.Exit(m.Run())
}
