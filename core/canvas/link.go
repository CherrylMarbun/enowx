package canvas

import (
	"strconv"
	"strings"
)

// LinkScheme starts a link to a canvas, or one of its designs, which notes
// and chats carry and the app opens in its Canvas page:
// canvas://<id>/design-3.
const LinkScheme = "canvas://"

// Link is the link to design n (from 1) of canvas id; n 0 for the canvas.
func Link(id string, n int) string {
	if n <= 0 {
		return LinkScheme + id
	}
	return LinkScheme + id + "/design-" + strconv.Itoa(n)
}

// ParseLink reads a canvas link: the canvas, and the design it names ("" for
// none) as FrameIndex takes it.
func ParseLink(link string) (id, design string, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(link), LinkScheme)
	if !found {
		return "", "", false
	}
	id, design, _ = strings.Cut(strings.Trim(rest, "/"), "/")
	if !idPattern.MatchString(id) {
		return "", "", false
	}
	return id, design, true
}
