package notesview

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// What a page's embeds show, fetched once each from where they point, off
// the main thread: an image, or a page's title for its link's card.

// fetchPoll is how often a page looks again while an embed loads.
const fetchPoll = 250 * time.Millisecond

// remote is an embed's link as fetched so far.
type remote struct {
	done  bool
	img   *ui.Bitmap
	title string
	err   string
}

// size is the image's size, in pixels.
func (r remote) size() (float32, float32) {
	if r.img == nil {
		return 0, 0
	}
	w, h := r.img.Size()
	return float32(w), float32(h)
}

var remotes struct {
	sync.Mutex
	m map[string]*remote
}

var fetchClient = &http.Client{Timeout: 15 * time.Second}

// fetch is link as fetched so far, fetching it the first time it is
// asked for: an image when image, else the title of the page there.
func fetch(link string, image bool) remote {
	remotes.Lock()
	defer remotes.Unlock()
	if remotes.m == nil {
		remotes.m = map[string]*remote{}
	}
	if r, ok := remotes.m[link]; ok {
		return *r
	}
	r := &remote{}
	remotes.m[link] = r
	go func() {
		got := load(link, image)
		got.done = true
		remotes.Lock()
		*r = got
		remotes.Unlock()
	}()
	return *r
}

func load(link string, image bool) remote {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return remote{err: "only http(s) links are fetched"}
	}
	resp, err := fetchClient.Get(link)
	if err != nil {
		return remote{err: "unreachable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return remote{err: fmt.Sprintf("the server answered %d", resp.StatusCode)}
	}
	limit := int64(512 << 10)
	if image {
		limit = 16 << 20
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return remote{err: "cut off"}
	}
	if image {
		img, err := ui.DecodeBitmap(data)
		if err != nil {
			return remote{err: "not an image this can show"}
		}
		return remote{img: img}
	}
	if m := titleRe.FindSubmatch(data); m != nil {
		return remote{title: strings.TrimSpace(html.UnescapeString(string(m[1])))}
	}
	return remote{}
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// isImage says link names an image by its extension.
func isImage(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

// hostOf is link's host, or link when it has none.
func hostOf(link string) string {
	if u, err := url.Parse(link); err == nil && u.Host != "" {
		return strings.TrimPrefix(u.Host, "www.")
	}
	return link
}
