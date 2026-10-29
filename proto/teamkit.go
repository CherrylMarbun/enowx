package proto

// A team's kit (docs/cloud.md): the skills, commands and subagents it
// shares, and the features its projects' sessions get, kept in its space
// and synced with it. A session in one of the team's projects is told of
// them, whatever its agent.
const (
	MethodTeamKit       = "team.kit"        // → []TeamKitItem, every team's
	MethodTeamKitAdd    = "team.kit.add"    // TeamKitAddParams → TeamKitItem (members and above)
	MethodTeamKitRemove = "team.kit.remove" // TeamKitRef (members and above)
	// MethodTeamFeatures sets the features of the team's projects'
	// sessions: on or off each, nil to leave one as the host has it.
	MethodTeamFeatures = "team.features" // TeamFeaturesParams → TeamFeaturesParams
)

// EventTeamKitChanged carries the full []TeamKitItem after a change.
const EventTeamKitChanged = "team.kit.changed"

// TeamKitItem is a skill, command or subagent of a team's kit.
type TeamKitItem struct {
	Team string `json:"team"`
	Kind string `json:"kind"` // EnvSkill, EnvCommand or EnvSubagent
	Name string `json:"name"`
	// About is what it is for, from its description.
	About string `json:"about,omitempty"`
	// Path is where it is on the host: a skill's folder, a file.
	Path string `json:"path"`
}

// TeamKitAddParams adds to Team's kit the host's agent item Item (an
// EnvItem's ID, as env.scan lists them).
type TeamKitAddParams struct {
	Team string `json:"team"`
	Item string `json:"item"`
}

// TeamKitRef names an item of a team's kit.
type TeamKitRef struct {
	Team string `json:"team"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// TeamFeaturesParams is a team's features for its projects' sessions;
// Set says to write them (admins), else they are read.
type TeamFeaturesParams struct {
	Team     string          `json:"team"`
	Features map[string]bool `json:"features,omitempty"`
	Set      bool            `json:"set,omitempty"`
}
