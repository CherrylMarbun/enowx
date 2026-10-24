package proto

// MethodSessionFanOut gives one prompt to several agents, each in a
// session on a new branch of its own, grouped to compare them and keep
// the best: FanOutParams → []SessionInfo.
const MethodSessionFanOut = "session.fanOut"

// FanOutParams are the sessions' common parameters, with BranchMode
// BranchNew, and the agents, one session each (an agent may repeat).
// Each branch is Branch with the agent's name after it ("fix-login-codex",
// "fix-login-claude-2").
type FanOutParams struct {
	SessionCreateParams
	Agents []string `json:"agents"`
}
