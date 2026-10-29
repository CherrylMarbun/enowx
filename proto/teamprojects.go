package proto

import "time"

// A team's projects (docs/cloud.md): repositories added to the team by
// their Git link, kept in its space and synced with it. A member's host
// finds its copy of one by its Git remote, among its projects and its
// sessions' folders, or clones it; the team's notes attached to it show
// under it there.
const (
	MethodTeamProjects      = "team.projects"       // → []TeamProject, every team's
	MethodTeamProjectAdd    = "team.project.add"    // TeamProjectAddParams → TeamProject (a team's admins)
	MethodTeamProjectRemove = "team.project.remove" // TeamProjectRef (a team's admins)
)

// EventTeamProjectsChanged carries the full []TeamProject after a change:
// one added or removed, or a copy of one found on the host.
const EventTeamProjectsChanged = "team.projects.changed"

// TeamProject is a project of a team.
type TeamProject struct {
	ID   string `json:"id"`
	Team string `json:"team"`
	Name string `json:"name"`
	// Repo is its Git link, as added: what a member clones. Ref names it
	// as notes attached to it do (NoteProject.Ref, "git:host/path").
	Repo    string    `json:"repo"`
	Ref     string    `json:"ref"`
	Added   time.Time `json:"added"`
	AddedBy string    `json:"addedBy,omitempty"` // the account's name
	// Here are its copies on this host; none when it isn't cloned here.
	Here []TeamProjectCopy `json:"here,omitempty"`
}

// TeamProjectCopy is a folder of the host holding a team's project: one
// of its projects, or a session's folder that isn't (Session names it).
type TeamProjectCopy struct {
	Path    string `json:"path"`
	Project bool   `json:"project,omitempty"` // a project of the host's
	Session string `json:"session,omitempty"`
}

// TeamProjectAddParams adds Repo, a Git link without credentials in it,
// to Team, named Name (the repository's when "").
type TeamProjectAddParams struct {
	Team string `json:"team"`
	Repo string `json:"repo"`
	Name string `json:"name,omitempty"`
}

// TeamProjectRef names a team's project.
type TeamProjectRef struct {
	Team string `json:"team"`
	ID   string `json:"id"`
}
