package ui

import (
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
)

// floatingLocation is one level of the menu, owned by the floating view.
type floatingLocation struct {
	mode, source    int
	hostID, project string
	page            int
}

func (s *floatingState) enter(next floatingLocation) {
	s.history = append(s.history, floatingLocation{s.mode, s.source, s.hostID, s.project, s.page})
	s.mode, s.source, s.hostID, s.project, s.page = next.mode, next.source, next.hostID, next.project, next.page
	s.fan, s.changed = "", true
}

func (s *floatingState) back() {
	if len(s.history) == 0 {
		*s = floatingState{source: -1, changed: true}
		return
	}
	last := s.history[len(s.history)-1]
	s.history = s.history[:len(s.history)-1]
	s.mode, s.source, s.hostID, s.project, s.page = last.mode, last.source, last.hostID, last.project, last.page
	s.fan, s.changed = "", true
}

func (a *App) floatingServers() []hosts.Host {
	var servers []hosts.Host
	for _, h := range a.allHosts() {
		if h.ID != hosts.LocalID && !h.IsDevice() {
			servers = append(servers, h)
		}
	}
	return servers
}

func (a *App) floatingChooseSource(st *floatingState, source int) {
	next := floatingLocation{mode: floatingProjectsMode, source: source, hostID: hosts.LocalID}
	if source == floatingVPS {
		servers := a.floatingServers()
		next.hostID = ""
		if len(servers) > 1 {
			next.mode = floatingHostsMode
		} else if len(servers) == 1 {
			next.hostID = servers[0].ID
		}
	}
	// Older daemons and unregistered shell sessions may have no project
	// metadata. Keep those sessions reachable without an empty extra step.
	if next.mode == floatingProjectsMode && len(a.floatingProjects(next.hostID, source)) == 0 {
		next.mode = floatingSessionsMode
	}
	st.enter(next)
}

func (a *App) floatingHostSessions(hostID string, source int) []proto.SessionInfo {
	if hostID == "" {
		return a.floatingSourceSessions(source)
	}
	if hostID == a.host.ID || (hostID == hosts.LocalID && a.host.ID == "") {
		if len(a.sessions) > 0 || a.watches[hostID] == nil {
			return a.sessions
		}
	}
	if watch := a.watches[hostID]; watch != nil {
		return watch.status.sessions
	}
	return nil
}

func floatingProjectPath(s proto.SessionInfo) string {
	if s.Project != "" {
		return s.Project
	}
	return s.Dir
}

func (a *App) floatingProjects(hostID string, source int) []proto.ProjectInfo {
	var projects []proto.ProjectInfo
	if hostID == a.host.ID || (hostID == hosts.LocalID && a.host.ID == "") {
		projects = slices.Clone(a.projects)
	} else if watch := a.watches[hostID]; watch != nil {
		projects = slices.Clone(watch.status.projects)
	}
	seen := make(map[string]bool, len(projects))
	for _, project := range projects {
		seen[project.Path] = true
	}
	for _, session := range a.floatingHostSessions(hostID, source) {
		projectPath := floatingProjectPath(session)
		if !seen[projectPath] {
			seen[projectPath] = true
			name := path.Base(strings.ReplaceAll(projectPath, `\`, "/"))
			if projectPath == "" {
				name = "Other sessions"
			}
			projects = append(projects, proto.ProjectInfo{Path: projectPath, Name: name})
		}
	}
	// A list consisting only of sessions with no folder needs no grouping.
	if len(projects) == 1 && projects[0].Path == "" {
		return nil
	}
	return projects
}

func (a *App) floatingLocationSessions(st *floatingState) []proto.SessionInfo {
	all := a.floatingHostSessions(st.hostID, st.source)
	if len(st.history) == 0 || st.history[len(st.history)-1].mode != floatingProjectsMode {
		return all
	}
	var sessions []proto.SessionInfo
	for _, session := range all {
		if floatingProjectPath(session) == st.project {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// floatingChoices pages server and project pickers rather than dropping
// choices when there are more folders than fit comfortably around the hub.
func (a *App) floatingChoices(st *floatingState) []floatingAction {
	var choices []floatingAction
	if st.mode == floatingHostsMode {
		for _, host := range a.floatingServers() {
			name := host.Name
			if name == "" {
				name = host.Address
			}
			choices = append(choices, floatingAction{key: "host:" + host.ID, label: name, short: floatingClip(name, 18), svg: icon.Server, enabled: true,
				navigate: func(s *floatingState) {
					mode := floatingProjectsMode
					if len(a.floatingProjects(host.ID, s.source)) == 0 {
						mode = floatingSessionsMode
					}
					s.enter(floatingLocation{mode: mode, source: s.source, hostID: host.ID})
				}})
		}
	} else {
		for _, project := range a.floatingProjects(st.hostID, st.source) {
			name := project.Name
			if name == "" {
				name = path.Base(strings.ReplaceAll(project.Path, `\`, "/"))
			}
			choices = append(choices, floatingAction{key: "project:" + project.Path, label: name, short: floatingClip(name, 18), svg: icon.Folder, enabled: true,
				navigate: func(s *floatingState) {
					s.enter(floatingLocation{mode: floatingSessionsMode, source: s.source, hostID: s.hostID, project: project.Path})
				}})
		}
	}
	const perPage = 5
	pages := max(1, (len(choices)+perPage-1)/perPage)
	st.page = min(st.page, pages-1)
	start := st.page * perPage
	end := min(len(choices), start+perPage)
	out := slices.Clone(choices[start:end])
	if st.page > 0 {
		out = append(out, floatingAction{key: "previous", label: "Previous page", short: "Previous", svg: icon.ArrowLeft, enabled: true,
			navigate: func(s *floatingState) { s.page--; s.changed = true }})
	}
	if st.page+1 < pages {
		out = append(out, floatingAction{key: "next", label: "Next page", short: "Next " + strconv.Itoa(st.page+2), svg: icon.ArrowRight, enabled: true,
			navigate: func(s *floatingState) { s.page++; s.changed = true }})
	}
	return out
}
