// Package quietcmd starts programs in the background without a console
// window. On Windows a program without a console of its own (the app, its
// daemon, which runs detached) that starts a console program (git, cmd,
// gh, npx…) gets a new console window for it, which flashes over the
// desktop; elsewhere nothing shows, and these do nothing more than
// os/exec's.
package quietcmd

import (
	"context"
	"os/exec"
)

// Command is exec.Command, its console hidden.
func Command(name string, args ...string) *exec.Cmd {
	return Hide(exec.Command(name, args...))
}

// CommandContext is exec.CommandContext, its console hidden.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return Hide(exec.CommandContext(ctx, name, args...))
}
