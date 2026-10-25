package setup

import (
	"context"
	"os/exec"

	"github.com/enowx/enowx/internal/quietcmd"
)

// scanCommand runs a tool probe in the background, without a console
// window on Windows.
func scanCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	return quietcmd.CommandContext(ctx, name, args...)
}
