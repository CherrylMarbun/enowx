package daemon

import "os"

// parentAgentMarkers are what an agent sets in the programs it runs, to
// know them as its own. A daemon started under one (the app opened from
// an agent's terminal, say) would hand them to every session: Claude Code
// there would take itself for that agent's child and keep no transcript,
// which the chat view and the Notes page read.
var parentAgentMarkers = []string{
	"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_CHILD_SESSION", "CLAUDE_CODE_SESSION_ID",
	"CLAUDE_CODE_SESSION_ATTENDED", "CLAUDE_CODE_MESSAGING_SOCKET", "CLAUDE_CODE_MESSAGING_TOKEN",
	"CLAUDE_CODE_EXECPATH", "CLAUDE_CODE_SSE_PORT", "CLAUDE_PID",
	"CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED", "CODEX_THREAD_ID",
}

// forgetParentAgent drops parentAgentMarkers from the daemon's own
// environment, which its sessions start from.
func forgetParentAgent() {
	for _, k := range parentAgentMarkers {
		_ = os.Unsetenv(k)
	}
}
