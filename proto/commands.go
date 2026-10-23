package proto

// Commands: what users run in a project again and again (its dev server,
// its build), found in its files or saved, each run in a new tab of a
// session, in a shell.
const (
	MethodCommandsList = "commands.list" // SessionRef → []RunCommand
	MethodCommandsRun  = "commands.run"  // CommandRunParams → SessionInfo
	MethodCommandsSave = "commands.save" // CommandSaveParams → []RunCommand
)

// Where a command comes from (RunCommand.Source): saved by the user, run
// lately, or found in the project's files.
const (
	CommandSaved  = "saved"
	CommandRecent = "recent"
)

// RunCommand is a command of a project, by name.
type RunCommand struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	// Source is CommandSaved, CommandRecent, or the file it was found in:
	// "package.json", "Makefile", "go.mod", "Cargo.toml", "compose".
	Source string `json:"source,omitempty"`
}

// CommandRunParams runs Command in a new tab of Session (of its session,
// when Session is a tab), and remembers it among the project's recent
// commands.
type CommandRunParams struct {
	Session string     `json:"session"`
	Command RunCommand `json:"command"`
}

// CommandSaveParams saves Command for Session's project, or with Remove
// forgets it, saved or recent.
type CommandSaveParams struct {
	Session string     `json:"session"`
	Command RunCommand `json:"command"`
	Remove  bool       `json:"remove,omitempty"`
}
