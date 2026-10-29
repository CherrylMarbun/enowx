package proto

import "encoding/json"

// A team's kit (docs/cloud.md): the skills, commands and subagents it
// shares, kept in its space and synced with it. A session in one of the team's projects is told of
// them, whatever its agent.
const (
	MethodTeamKit       = "team.kit"        // → []TeamKitItem, every team's
	MethodTeamKitAdd    = "team.kit.add"    // TeamKitAddParams → TeamKitItem (members and above)
	MethodTeamKitRemove = "team.kit.remove" // TeamKitRef (members and above)
	// MethodTeamKitSave keeps a definition in the kit: a pipeline's
	// stages, an automation, a project template.
	MethodTeamKitSave = "team.kit.save" // TeamKitSaveParams → TeamKitItem (members and above)
)

// EventTeamKitChanged carries the full []TeamKitItem after a change.
const EventTeamKitChanged = "team.kit.changed"

// Kinds of a kit's definitions, beside its skills, commands and subagents.
const (
	KitPipeline   = "pipeline"   // Data: []PipelineStage
	KitAutomation = "automation" // Data: Automation, its Project a team project's ref
	KitTemplate   = "template"   // Data: KitTemplateData
)

// KitTemplateData is a project template: a repository new projects start
// from, cloned.
type KitTemplateData struct {
	Repo string `json:"repo"`
}

// TeamKitItem is a skill, command or subagent of a team's kit, or a
// definition (KitPipeline, KitAutomation, KitTemplate) with its Data.
type TeamKitItem struct {
	Team string          `json:"team"`
	Kind string          `json:"kind"` // EnvSkill, EnvCommand, EnvSubagent, or a Kit kind
	Data json.RawMessage `json:"data,omitempty"`
	Name string          `json:"name"`
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

// TeamKitSaveParams keeps definition Name of Kind in Team's kit.
type TeamKitSaveParams struct {
	Team  string          `json:"team"`
	Kind  string          `json:"kind"`
	Name  string          `json:"name"`
	About string          `json:"about,omitempty"`
	Data  json.RawMessage `json:"data"`
}

// TeamKitRef names an item of a team's kit.
type TeamKitRef struct {
	Team string `json:"team"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}
