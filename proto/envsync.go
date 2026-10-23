package proto

// Environment sync: the agents' skills, subagents, commands, memory files
// and MCP servers, copied item by item from one daemon's home to another's.
const (
	MethodEnvScan   = "env.scan"   // → EnvScan
	MethodEnvExport = "env.export" // EnvItemRef → EnvPayload
	MethodEnvImport = "env.import" // EnvPayload → EnvImportResult
	MethodEnvDelete = "env.delete" // EnvItemRef
)

// Kinds of environment items.
const (
	EnvSkill    = "skill"
	EnvSubagent = "subagent"
	EnvCommand  = "command"
	EnvMemory   = "memory"
	EnvSettings = "settings"
	EnvMCP      = "mcp"
	EnvGit      = "git"
)

// EnvScan is what a daemon's home holds.
type EnvScan struct {
	Home  string    `json:"home"`
	Items []EnvItem `json:"items"`
}

// EnvItem is one thing that can be synced.
type EnvItem struct {
	ID     string `json:"id"`    // "claude/skill/review"; the same on every host
	Agent  string `json:"agent"` // an agent preset ID, or "shared"
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"` // a path under ~, or what an MCP server runs
	// Hash compares the item between hosts; paths under the home folder
	// don't count, so the same skill hashes the same everywhere.
	Hash string `json:"hash"`
	Size int64  `json:"size,omitempty"`
	// Secrets names what in the item looks secret (env variables,
	// headers, key files). Such items are sent only when the user asks.
	Secrets []string `json:"secrets,omitempty"`
	// Warnings say why the item may not work as is on another host.
	Warnings []string `json:"warnings,omitempty"`
}

// EnvItemRef names an item.
type EnvItemRef struct {
	ID string `json:"id"`
}

// EnvPayload is an item's content, as exported by one daemon and imported
// by another.
type EnvPayload struct {
	Item EnvItem `json:"item"`
	Home string  `json:"home"` // the exporting home, rewritten to the importing one
	// Files are the item's files, by slash-separated path under the home.
	Files []EnvFile `json:"files,omitempty"`
	// Value is a config entry: an MCP server's JSON, or the git identity.
	Value []byte `json:"value,omitempty"`
}

// EnvFile is a file of an item.
type EnvFile struct {
	Path string `json:"path"`
	Mode uint32 `json:"mode"`
	Data []byte `json:"data"`
}

// EnvImportResult says how an import went.
type EnvImportResult struct {
	Warnings []string `json:"warnings,omitempty"`
}
