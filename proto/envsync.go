package proto

// Environment sync: the agents' skills, subagents, commands, memory files
// and MCP servers, copied item by item from one daemon's home to another's.
const (
	MethodEnvScan   = "env.scan"   // → EnvScan
	MethodEnvExport = "env.export" // EnvItemRef → EnvPayload
	MethodEnvImport = "env.import" // EnvPayload → EnvImportResult
	MethodEnvDelete = "env.delete" // EnvItemRef

	// A project's own items, under its folder (CLAUDE.md, .mcp.json,
	// .claude/commands, …): the same, with the project named.
	MethodEnvProjectScan   = "env.project.scan"   // EnvProjectRef → EnvScan
	MethodEnvProjectExport = "env.project.export" // EnvProjectRef → EnvPayload
	MethodEnvProjectImport = "env.project.import" // EnvProjectImport → EnvImportResult
	MethodEnvProjectDelete = "env.project.delete" // EnvProjectRef

	// MethodEnvMCPTest runs an MCP server where its agent would, and lists
	// its tools.
	MethodEnvMCPTest = "env.mcp.test" // EnvProjectRef (Project "" for the user's) → MCPTest

	// Skills from a git repository: fetched (cloned on the host, its
	// skills listed), then those picked added.
	MethodEnvSkillFetch   = "env.skill.fetch"   // SkillFetchParams → SkillFetch
	MethodEnvSkillInstall = "env.skill.install" // SkillInstallParams → []string (the skills added)
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

// EnvProjectRef names a project, and an item of it for export and delete.
type EnvProjectRef struct {
	Project string `json:"project"`
	ID      string `json:"id,omitempty"`
}

// EnvProjectImport writes an item into a project.
type EnvProjectImport struct {
	Project string     `json:"project"`
	Payload EnvPayload `json:"payload"`
}

// MCPTest is how a test of an MCP server went.
type MCPTest struct {
	OK       bool      `json:"ok"`
	Server   string    `json:"server,omitempty"`   // its name and version, as it says
	Protocol string    `json:"protocol,omitempty"` // the MCP version it speaks
	Tools    []MCPTool `json:"tools,omitempty"`
	Error    string    `json:"error,omitempty"`
	Stderr   string    `json:"stderr,omitempty"` // the end of what it printed
	Took     int64     `json:"took"`             // milliseconds
}

// MCPTool is a tool an MCP server offers.
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SkillFetchParams fetches the repository at URL ("https://github.com/o/r",
// or .../tree/<branch>/<folder>).
type SkillFetchParams struct {
	URL string `json:"url"`
}

// SkillFetch is a fetched repository's skills, kept on the host as ID
// until installed from.
type SkillFetch struct {
	ID     string       `json:"id"`
	Skills []FoundSkill `json:"skills"`
}

// FoundSkill is a skill found in a repository.
type FoundSkill struct {
	Path        string `json:"path"` // its folder in the repository
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SkillInstallParams adds the skills at Paths of fetch ID to Agent, the
// user's or Project's.
type SkillInstallParams struct {
	ID      string   `json:"id"`
	Paths   []string `json:"paths"`
	Agent   string   `json:"agent"`
	Project string   `json:"project,omitempty"`
}
