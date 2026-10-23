package daemon

import "strings"

// thenShell runs argv and, once it exits, leaves PowerShell running in the
// same terminal, as when an agent is started from a terminal app.
func thenShell(argv []string) []string {
	if len(argv) == 0 {
		return nil
	}
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", "''") + "'"
	}
	return []string{"powershell.exe", "-NoLogo", "-NoExit", "-Command", "& " + strings.Join(quoted, " ")}
}

// scriptArgv runs a shell script: a project's worktree setup, say.
func scriptArgv(script string) []string { return []string{"cmd", "/c", script} }
