package proto

import "time"

// Hooks run what the user set (a command, context for the agent, a
// refusal) when something happens in a session, on every agent: in the
// agent through its own hooks where it has them, else as the daemon sees
// it from outside. See docs/hooks.md.
const (
	MethodHooksList    = "hooks.list"    // ProjectDir → HooksInfo
	MethodHooksSave    = "hooks.save"    // []Hook: the host's own hooks, all of them
	MethodHooksApprove = "hooks.approve" // HooksApproval: a project's hooks allowed to run
	MethodHooksTest    = "hooks.test"    // HookTest → HookRun, run now on a sample event
	MethodHooksLog     = "hooks.log"     // → []HookRun, the latest first
	// MethodSessionHook is how an agent's own hooks hand the daemon an
	// event (enowx hook --event): it runs the hooks that match and answers
	// what the agent is to do.
	MethodSessionHook = "session.hook" // SessionHookParams → HookDecision
)

// Hook events: when a hook runs.
const (
	HookSessionStart = "session.start" // the agent started, or was continued
	HookSessionEnd   = "session.end"   // its program ended
	HookPromptStart  = "prompt.start"  // a prompt is about to reach the agent
	HookTurnDone     = "turn.done"     // the agent finished its turn
	HookToolBefore   = "tool.before"   // a tool is about to run
	HookToolAfter    = "tool.after"    // a tool ran
	HookPermission   = "permission"    // the agent asks for permission
	HookWaiting      = "waiting"       // the agent waits for the user
	HookFileChanged  = "file.changed"  // files changed in a turn
	HookCommit       = "git.commit"    // a commit was made in the session's folder
)

// HookEvents are the events, in the order they're offered.
var HookEvents = []string{HookSessionStart, HookPromptStart, HookToolBefore, HookToolAfter, HookPermission, HookWaiting, HookTurnDone, HookFileChanged, HookCommit, HookSessionEnd}

// Hook actions: what a hook does.
const (
	HookActionRun     = "run"     // run Command, the event as JSON on its stdin
	HookActionContext = "context" // add Context to what the agent reads
	HookActionBlock   = "block"   // refuse the action, Reason told to the agent
)

// How an agent gets an event (HookSupport).
const (
	HookInAgent = "agent"   // from its own hooks: it can be refused, given context
	HookOutside = "outside" // seen by the daemon, after the fact: nothing to refuse
)

// Hook is something to do when an event happens.
type Hook struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	On   bool   `json:"on"`
	// Event is when (Hook* events).
	Event string `json:"event"`
	// When it may run: every one set must match. Agents are base IDs
	// ("claude"); Projects folders or their names; Tool a regular
	// expression on the tool's name or command; Paths globs on the files
	// changed or touched; Text a regular expression on the prompt or the
	// command. Empty matches all.
	Agents   []string `json:"agents,omitempty"`
	Projects []string `json:"projects,omitempty"`
	Tool     string   `json:"tool,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Text     string   `json:"text,omitempty"`
	// Action is what it does (HookActionRun, HookActionContext,
	// HookActionBlock).
	Action  string `json:"action"`
	Command string `json:"command,omitempty"`
	Context string `json:"context,omitempty"`
	Reason  string `json:"reason,omitempty"`
	// Background runs Command without the agent waiting for it, so it can
	// neither refuse nor add context.
	Background bool `json:"background,omitempty"`
	// Timeout bounds Command, in seconds (0: 30).
	Timeout int `json:"timeout,omitempty"`
	// Project is the folder of a project's hook (its .enowx/hooks.json),
	// "" for the host's own; filled in by the daemon.
	Project string `json:"project,omitempty"`
}

// HookEvent is what happened, handed to a hook's command as JSON.
type HookEvent struct {
	Event   string `json:"event"`
	Session string `json:"session,omitempty"`
	Agent   string `json:"agent,omitempty"`
	// Dir is the session's folder; Project the project's, Branch its
	// branch.
	Dir     string `json:"dir,omitempty"`
	Project string `json:"project,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
	// Tool is the tool's name, Command its command line or what it was
	// given, Failed that it failed.
	Tool    string `json:"tool,omitempty"`
	Command string `json:"command,omitempty"`
	Failed  bool   `json:"failed,omitempty"`
	// Paths are the files changed (HookFileChanged) or the tool touched.
	Paths []string `json:"paths,omitempty"`
	// Commit is the commit made (HookCommit), Message its subject.
	Commit  string `json:"commit,omitempty"`
	Message string `json:"message,omitempty"`
	// Via is how it was seen (HookInAgent, HookOutside).
	Via  string    `json:"via,omitempty"`
	Time time.Time `json:"time"`
}

// HookDecision is what the agent is to do after the hooks ran: go on,
// refuse with Reason, with Context to read.
type HookDecision struct {
	Block   bool   `json:"block,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Context string `json:"context,omitempty"`
}

// SessionHookParams is an event from session ID's agent, with the state
// it means for the session (State*, "" for none).
type SessionHookParams struct {
	ID    string    `json:"id"`
	State string    `json:"state,omitempty"`
	Event HookEvent `json:"event"`
}

// HookRun is a hook's run: what it was given and what came of it.
type HookRun struct {
	Hook    string    `json:"hook"`
	Name    string    `json:"name"`
	Event   string    `json:"event"`
	Session string    `json:"session,omitempty"`
	Agent   string    `json:"agent,omitempty"`
	Start   time.Time `json:"start"`
	Ms      int64     `json:"ms"`
	Exit    int       `json:"exit"`
	// Output is the end of what its command printed; Error why it didn't
	// run or failed (a timeout, say); Blocked that it refused the action.
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
	Blocked bool   `json:"blocked,omitempty"`
}

// ProjectDir names a project's folder, "" for none.
type ProjectDir struct {
	Dir string `json:"dir,omitempty"`
}

// HooksInfo is the host's hooks and those of projects, the ones waiting
// for the user's approval apart, and how each agent gets each event.
type HooksInfo struct {
	Hooks   []Hook         `json:"hooks"`
	Pending []ProjectHooks `json:"pending,omitempty"`
	// Support is, per agent base ID, how it gets each event (HookInAgent,
	// HookOutside); an event missing it doesn't get.
	Support map[string]map[string]string `json:"support,omitempty"`
}

// ProjectHooks are the hooks of a project's .enowx/hooks.json, as one
// approval: Hash is of the file, so a change asks again.
type ProjectHooks struct {
	Dir   string `json:"dir"`
	Hash  string `json:"hash"`
	Hooks []Hook `json:"hooks"`
}

// HooksApproval allows (or, Allow false, stops allowing) the hooks of
// project Dir's file with Hash.
type HooksApproval struct {
	Dir   string `json:"dir"`
	Hash  string `json:"hash"`
	Allow bool   `json:"allow"`
}

// HookTest runs Hook on Event now, as if it happened.
type HookTest struct {
	Hook  Hook      `json:"hook"`
	Event HookEvent `json:"event"`
}
