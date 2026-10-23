package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/ui/textfmt"
)

// Files dropped on a server's session: the terminal pastes their paths on
// this machine, which the server doesn't have. uploadConn sits between the
// terminal and the session's stream, and turns such a paste into the
// files' paths on the server after uploading them there.

// maxUpload bounds the files one paste uploads, in bytes.
const maxUpload = 200 << 20

// uploadConn is a remote session's stream, with dropped files uploaded.
// The stream's other methods (resizing, …) are its own.
type uploadConn struct {
	*client.Stream
	upload func(path string) (string, error) // to the session's host
	report func(msg string, failed bool)     // on the main thread's terms
}

// Write passes p on, its local file paths replaced by the uploaded files'.
func (u *uploadConn) Write(p []byte) (int, error) {
	if paths, rewrite, ok := pastedFiles(p); ok {
		remote := make([]string, len(paths))
		u.report("Uploading "+textfmt.Plural(len(paths), "a file", "files")+" to the server…", false)
		for i, path := range paths {
			r, err := u.upload(path)
			if err != nil {
				u.report("Couldn't upload "+filepath.Base(path)+": "+err.Error(), true)
				return len(p), nil // pasting a path the server lacks would mislead
			}
			remote[i] = r
		}
		u.report("Uploaded "+textfmt.Plural(len(paths), "a file", "files")+" to the server", false)
		if _, err := u.Stream.Write(rewrite(remote)); err != nil {
			return 0, err
		}
		return len(p), nil
	}
	return u.Stream.Write(p)
}

// Bracketed paste markers, which a program that asked for them gets
// around what is pasted.
var (
	pasteStart = []byte("\x1b[200~")
	pasteEnd   = []byte("\x1b[201~")
)

// pastedFiles reports whether p is a paste of local files' paths only, as
// the terminal pastes files dropped on it: each escaped for a POSIX shell,
// spaces between. It returns them, and how to write the same paste with
// other paths.
func pastedFiles(p []byte) ([]string, func([]string) []byte, bool) {
	body, bracketed := p, false
	if bytes.HasPrefix(p, pasteStart) && bytes.HasSuffix(p, pasteEnd) {
		body, bracketed = p[len(pasteStart):len(p)-len(pasteEnd)], true
	}
	paths := splitEscaped(string(body))
	if len(paths) == 0 {
		return nil, nil, false
	}
	var total int64
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return nil, nil, false
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, nil, false
		}
		total += info.Size()
	}
	if total > maxUpload {
		return nil, nil, false
	}
	rewrite := func(remote []string) []byte {
		escaped := make([]string, len(remote))
		for i, r := range remote {
			escaped[i] = escapePath(r)
		}
		out := []byte(strings.Join(escaped, " "))
		if bracketed {
			out = append(append(append([]byte{}, pasteStart...), out...), pasteEnd...)
		}
		return out
	}
	return paths, rewrite, true
}

// splitEscaped splits s at unescaped spaces, undoing backslash escapes;
// nil when s isn't only such words (a newline, quotes).
func splitEscaped(s string) []string {
	if s == "" || strings.ContainsAny(s, "\n\r'\"") {
		return nil
	}
	var out []string
	var cur strings.Builder
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ' ':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if escaped {
		return nil
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// escapePath escapes a path for a POSIX shell, as the terminal does.
func escapePath(path string) string {
	var b strings.Builder
	for _, r := range path {
		if strings.ContainsRune("\\ ()[]{}<>\"'`!#$&;|*?\t", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
