package notes

import (
	"net/url"
	"strings"

	"github.com/enowx/enowx/internal/hostpath"
)

// AssetReference recognizes portable references and full paths to notes' assets.
// Full paths become portable so synced notes work on another host as well.
func AssetReference(link string) (string, bool) {
	if strings.HasPrefix(link, "<") && strings.HasSuffix(link, ">") {
		link = link[1 : len(link)-1]
	}
	p := link
	if !hostpath.Windows(p) {
		if u, err := url.Parse(p); err == nil && u.Scheme != "" {
			if u.Scheme != "file" {
				return "", false
			}
			p = hostpath.FileURL(u)
		}
	}
	p = strings.ReplaceAll(p, `\`, "/")
	if hostpath.IsAbs(p) {
		marker := "/notes/assets/"
		at := strings.LastIndex(strings.ToLower(p), marker)
		if at < 0 {
			return "", false
		}
		p = "assets/" + p[at+len(marker):]
	}
	rest, ok := strings.CutPrefix(p, "assets/")
	id, name, split := strings.Cut(rest, "/")
	if !ok || !split || !idPattern.MatchString(id) || name == "" || name != assetName(name) {
		return "", false
	}
	return "assets/" + id + "/" + name, true
}
