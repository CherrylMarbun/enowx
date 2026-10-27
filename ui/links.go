package ui

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/enowx/enowx/internal/hostpath"
	"github.com/enowx/enowx/proto"
)

// Links a session's program prints open in the app with Command+click
// (Control+click elsewhere): a file in a tab of the side panel, wherever
// it is on the host (an image shows, Markdown renders), and a page of a
// session's dev server in the Browser tool. Other URLs open with the
// system.

// lineSuffix is the :line or :line:col a path may end in.
var lineSuffix = regexp.MustCompile(`(:\d+){1,2}$`)

// openLink opens link, printed by session id's program; it reports
// whether it did, leaving the rest to the terminal.
func (a *App) openLink(id, link string) bool {
	if strings.HasPrefix(link, "<") && strings.HasSuffix(link, ">") {
		link = link[1 : len(link)-1]
	}
	if a.openCanvasLink(link) {
		return true
	}
	s, ok := a.sessionByID(id)
	if !ok {
		return false
	}
	if !hostpath.Windows(link) {
		if u, err := url.Parse(link); err == nil && u.Scheme == "file" {
			link = hostpath.FileURL(u)
		} else if err == nil && u.Scheme != "" {
			return a.openLocalURL(s, u)
		}
	}
	p := a.linkPath(s, link)
	if p == "" {
		return false
	}
	top := a.parentOf(s)
	a.selectSession(top)
	a.openFileTab(top, p)
	return true
}

// openLocalURL previews a page on localhost in the Browser tool, as one of
// session s's ports.
func (a *App) openLocalURL(s proto.SessionInfo, u *url.URL) bool {
	host := u.Hostname()
	if (u.Scheme != "http" && u.Scheme != "https") || !webViewAvailable() ||
		(host != "localhost" && host != "127.0.0.1" && host != "0.0.0.0" && host != "::1") {
		return false
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return false
	}
	a.previewPort(a.parentOf(s), proto.Port{Port: port, Addr: "127.0.0.1"})
	return true
}

// linkPath is the file link names, as the Files tool names it: relative
// to session s's folder when inside it, else absolute on the host. ""
// when it can't tell.
func (a *App) linkPath(s proto.SessionInfo, link string) string {
	p := lineSuffix.ReplaceAllString(link, "")
	dir := hostpath.Clean(s.Dir)
	if hostpath.Windows(s.Dir) || hostpath.Windows(p) {
		p = strings.ReplaceAll(p, `\`, "/")
	}
	switch {
	case strings.HasPrefix(p, "~/"):
		if a.daemonInfo.Home == "" {
			return ""
		}
		p = hostpath.Clean(a.daemonInfo.Home) + "/" + p[2:]
	case !hostpath.IsAbs(p):
		if s.Dir == "" {
			return ""
		}
		p = dir + "/" + p
	}
	p = hostpath.Clean(p)
	dirPrefix := strings.TrimSuffix(dir, "/") + "/"
	inside := strings.HasPrefix(p, dirPrefix)
	if hostpath.Windows(dir) {
		inside = strings.HasPrefix(strings.ToLower(p), strings.ToLower(dirPrefix))
	}
	if s.Dir != "" && inside {
		return p[len(dirPrefix):]
	}
	return p
}

// sessionByID is the session id, if the app has it.
func (a *App) sessionByID(id string) (proto.SessionInfo, bool) {
	for _, s := range a.sessions {
		if s.ID == id {
			return s, true
		}
	}
	return proto.SessionInfo{}, false
}
