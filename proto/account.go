package proto

// An agent's accounts on a host: the agent signed in with each in a home
// of its own, picked per session, so switching needs no new sign-in.
// Agents that keep their sign-in in a folder a variable names (Claude
// Code, Codex) can have them; the default account is the agent's own.
const (
	MethodAccountList   = "account.list"   // → []Account
	MethodAccountAdd    = "account.add"    // AccountAddParams → Account
	MethodAccountRemove = "account.remove" // AccountRef
)

// Account is one of an agent's accounts.
type Account struct {
	ID    string `json:"id"`
	Agent string `json:"agent"` // the agent it signs in (its base: claude for Claude Agent Teams)
	Name  string `json:"name"`
	// Member is the cloud account of the team's member it is for, on a
	// host they visit: its home links nothing of the host's user's.
	Member string `json:"member,omitempty"`
}

// AccountAddParams name a new account of Agent.
type AccountAddParams struct {
	Agent string `json:"agent"`
	Name  string `json:"name"`
}

// AccountRef names an account.
type AccountRef struct {
	ID string `json:"id"`
}
