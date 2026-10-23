package ui

import (
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/proto"
)

// editor is a code editor enowX opens a session's folder or files in, by
// its URL scheme: on this machine directly, and on a server through the
// editor's own SSH remote (VS Code's and Cursor's Remote-SSH, Zed's SSH
// projects), so the files never leave the server.
type editor struct {
	id, name string
}

// editors are the editors offered, the default first.
var editors = []editor{{"vscode", "VS Code"}, {"cursor", "Cursor"}, {"zed", "Zed"}}

// editorURL is the URL that opens path (absolute, on host) at line (0 for
// none) in editor id. host is the SSH destination, "" for this machine.
func editorURL(id, host, path string, line int) string {
	segments := strings.Split(path, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	p := strings.Join(segments, "/")
	if line > 0 {
		p += ":" + strconv.Itoa(line)
	}
	switch {
	case id == "zed" && host == "":
		return "zed://file" + p
	case id == "zed":
		return "zed://ssh/" + host + p
	case host == "":
		return id + "://file" + p
	}
	// VS Code and Cursor, which share VS Code's remote URLs.
	return id + "://vscode-remote/ssh-remote+" + host + p
}

// preferredEditor is the editor last opened, or the default.
func (a *App) preferredEditor() editor {
	if i := slices.IndexFunc(editors, func(e editor) bool { return e.id == a.state.Editor }); i >= 0 {
		return editors[i]
	}
	return editors[0]
}

// openInEditor opens path, absolute on the connected host, in editor ed,
// and remembers ed for the next time.
func (a *App) openInEditor(ed editor, path string, line int) {
	host := ""
	if a.host.ID != hosts.LocalID {
		host = a.host.Address
	}
	a.state.Editor = ed.id
	u := editorURL(ed.id, host, path, line)
	go func() {
		if err := mygo.Shell.OpenExternal(u); err != nil {
			a.onMain(func() { a.status = "Couldn't open " + ed.name + ": " + err.Error() })
		}
	}()
}

// sessionFolder is the folder session s runs in, on its host.
func (a *App) sessionFolder(s proto.SessionInfo) string {
	if s.Dir != "" {
		return s.Dir
	}
	return s.Project
}
