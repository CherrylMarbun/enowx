// Package hostpath reads paths of a daemon's host, independent of the app's OS.
package hostpath

import (
	"net/url"
	"path"
	"strings"
)

// Windows reports whether p is a drive-qualified or UNC absolute path.
func Windows(p string) bool {
	return len(p) >= 3 && (p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z') && p[1] == ':' && (p[2] == '/' || p[2] == '\\') || strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//")
}

// IsAbs recognizes absolute paths on Unix and Windows hosts.
func IsAbs(p string) bool { return strings.HasPrefix(p, "/") || Windows(p) }

// Clean uses forward slashes for Windows paths. Its volume (a drive, or
// a UNC path's //host/share) stays: ".." stops at it, as on Windows.
func Clean(p string) string {
	if !Windows(p) {
		return path.Clean(p)
	}
	p = strings.ReplaceAll(p, `\`, "/")
	vol, rest := p[:2], p[2:]
	if strings.HasPrefix(p, "//") {
		parts := strings.SplitN(p[2:], "/", 3)
		vol, rest = "//"+strings.Join(parts[:min(2, len(parts))], "/"), ""
		if len(parts) > 2 {
			rest = parts[2]
		}
	}
	clean := path.Clean("/" + rest)
	switch {
	case clean != "/":
		return vol + clean
	case strings.HasPrefix(vol, "//"):
		return vol
	default:
		return vol + "/"
	}
}

// FileURL returns the host path represented by a file URL.
func FileURL(u *url.URL) string {
	p := u.Path
	if p == "" {
		p = u.Opaque
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "//" + u.Host + p
	}
	if strings.HasPrefix(p, "/") && Windows(p[1:]) {
		p = p[1:]
	}
	return p
}
