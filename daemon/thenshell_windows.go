package daemon

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/enowx/enowx/core/shells"
)

var backslashesBeforeQuote = regexp.MustCompile(`(\\*)(")`)

// psLegacy has PowerShell 7.3+ pass a program its arguments as Windows
// PowerShell 5.1 does, which psQuote escapes for: their own way ("Windows")
// would pass psQuote's escapes on as backslashes. Older PowerShells ignore
// it.
const psLegacy = "$PSNativeCommandArgumentPassing = 'Legacy'; "

// thenShell runs argv and, once it exits, leaves the shell terminals
// start with running in the same terminal, as when an agent is started
// from a terminal app. PowerShell runs argv: the one picked, staying on;
// for another shell (the Command Prompt, Git Bash, WSL), Windows' runs it
// after, as the Command Prompt can't be given a command line safely.
func thenShell(argv []string) []string {
	if len(argv) == 0 {
		return nil
	}
	run := psLegacy + "& " + psQuote(argv)
	shell := defaultShell()
	switch strings.ToLower(filepath.Base(shell)) {
	case "pwsh.exe", "powershell.exe":
		return []string{shell, "-NoLogo", "-NoExit", "-Command", run}
	case "":
		return []string{"powershell.exe", "-NoLogo", "-NoExit", "-Command", run}
	}
	return []string{"powershell.exe", "-NoLogo", "-Command", run + "; & " + psQuote(shells.Argv(shell))}
}

// psQuote is argv as PowerShell's words.
func psQuote(argv []string) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		if a == "" {
			quoted[i] = `'""'`
			continue
		}
		// In PowerShell, single quotes are escaped by doubling them (' -> '').
		// When PowerShell 5.1 passes arguments to a native executable, double
		// quotes are stripped unless escaped with \". Any preceding backslashes
		// must also be doubled so they don't escape the backslash.
		s := backslashesBeforeQuote.ReplaceAllString(a, `$1$1\"`)
		s = strings.ReplaceAll(s, "'", "''")
		quoted[i] = "'" + s + "'"
	}
	return strings.Join(quoted, " ")
}

// scriptArgv runs a shell script: a project's worktree setup, say.
func scriptArgv(script string) []string { return []string{"cmd", "/c", script} }
