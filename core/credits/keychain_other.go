//go:build !darwin

package credits

import "context"

// claudeKeychain is empty off macOS: Claude Code keeps its login in
// .credentials.json there.
func claudeKeychain(context.Context) [][]byte { return nil }
