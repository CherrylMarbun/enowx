package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/proto"
)

// Canvases shared by a link: the daemon serves each a read-only page,
// rendered from the canvas as it is when asked, on loopback and on the
// tailnet's address alone, never on other networks. The links live in
// canvas-shares.json beside the synced folders, not in them: a link is
// this host's to serve.

// shareFirstPort is the port tried first, so links outlive restarts.
const shareFirstPort = 47831

type canvasShares struct {
	mu        sync.Mutex
	loaded    bool
	file      shareFile
	listeners []net.Listener
	server    *http.Server
	port      int
}

type shareFile struct {
	Port   int                 `json:"port,omitempty"`
	Shares []proto.CanvasShare `json:"shares"`
}

func (s *Server) sharesPath() string { return filepath.Join(s.dataDir, "canvas-shares.json") }

// loadShares reads the links once; the caller holds sh.mu.
func (s *Server) loadShares() {
	sh := &s.shares
	if sh.loaded {
		return
	}
	sh.loaded = true
	if data, err := os.ReadFile(s.sharesPath()); err == nil {
		_ = json.Unmarshal(data, &sh.file)
	}
}

func (s *Server) saveShares() error {
	data, err := json.MarshalIndent(s.shares.file, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.sharesPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.sharesPath())
}

// resumeShares serves the links kept, when there are.
func (s *Server) resumeShares() {
	sh := &s.shares
	sh.mu.Lock()
	defer sh.mu.Unlock()
	s.loadShares()
	if len(sh.file.Shares) > 0 {
		_ = s.serveShares()
	}
}

// shareCanvas makes canvas id's link, or stops it.
func (s *Server) shareCanvas(p proto.CanvasShareParams) (proto.CanvasShare, error) {
	c, err := s.canvasFor(p.ID).Get(p.ID)
	if err != nil && !p.Stop {
		return proto.CanvasShare{}, err
	}
	sh := &s.shares
	sh.mu.Lock()
	defer sh.mu.Unlock()
	s.loadShares()
	i := slices.IndexFunc(sh.file.Shares, func(x proto.CanvasShare) bool { return x.Canvas == p.ID })
	if p.Stop {
		if i < 0 {
			return proto.CanvasShare{}, fmt.Errorf("canvas: %s has no link", p.ID)
		}
		sh.file.Shares = slices.Delete(sh.file.Shares, i, i+1)
		err := s.saveShares()
		if len(sh.file.Shares) == 0 {
			s.stopShares()
		}
		return proto.CanvasShare{}, err
	}
	if err := s.serveShares(); err != nil {
		return proto.CanvasShare{}, err
	}
	if i >= 0 {
		sh.file.Shares[i].Title = c.Title
		return s.withURLs(sh.file.Shares[i]), nil
	}
	var b [16]byte
	_, _ = rand.Read(b[:])
	x := proto.CanvasShare{Canvas: c.ID, Title: c.Title, Token: hex.EncodeToString(b[:]), Created: time.Now().UTC()}
	sh.file.Shares = append(sh.file.Shares, x)
	if err := s.saveShares(); err != nil {
		return proto.CanvasShare{}, err
	}
	return s.withURLs(x), nil
}

// canvasShareList is the links, each with its addresses.
func (s *Server) canvasShareList() []proto.CanvasShare {
	sh := &s.shares
	sh.mu.Lock()
	defer sh.mu.Unlock()
	s.loadShares()
	out := []proto.CanvasShare{}
	for _, x := range sh.file.Shares {
		out = append(out, s.withURLs(x))
	}
	return out
}

// withURLs is x with the addresses its page is served at; sh.mu held.
func (s *Server) withURLs(x proto.CanvasShare) proto.CanvasShare {
	x.URLs = nil
	for _, l := range s.shares.listeners {
		x.URLs = append(x.URLs, "http://"+l.Addr().String()+"/c/"+x.Token)
	}
	return x
}

// serveShares starts serving, if not yet: loopback, and the tailnet's
// address when this host is on one. sh.mu held.
func (s *Server) serveShares() error {
	sh := &s.shares
	if len(sh.listeners) > 0 {
		return nil
	}
	hosts := []string{"127.0.0.1"}
	if ip := tailnetIP(); ip != "" {
		hosts = append(hosts, ip)
	}
	port := sh.file.Port
	if port == 0 {
		port = shareFirstPort
	}
	listen := func(port int) ([]net.Listener, error) {
		var ls []net.Listener
		for _, h := range hosts {
			l, err := net.Listen("tcp", net.JoinHostPort(h, strconv.Itoa(port)))
			if err != nil {
				for _, o := range ls {
					o.Close()
				}
				return nil, err
			}
			ls = append(ls, l)
			port = l.Addr().(*net.TCPAddr).Port // the others take the first's
		}
		return ls, nil
	}
	ls, err := listen(port)
	if err != nil {
		if ls, err = listen(0); err != nil {
			return fmt.Errorf("canvas: serve links: %w", err)
		}
	}
	sh.listeners, sh.port = ls, ls[0].Addr().(*net.TCPAddr).Port
	if sh.file.Port != sh.port {
		sh.file.Port = sh.port
		_ = s.saveShares()
	}
	srv := &http.Server{Handler: http.HandlerFunc(s.serveShared), ReadHeaderTimeout: 10 * time.Second}
	sh.server = srv
	for _, l := range ls {
		go func() { _ = srv.Serve(l) }()
	}
	return nil
}

// stopShares stops serving, open connections too; sh.mu held.
func (s *Server) stopShares() {
	if s.shares.server != nil {
		_ = s.shares.server.Close()
	}
	for _, l := range s.shares.listeners {
		l.Close() // one Serve hasn't taken yet
	}
	s.shares.listeners, s.shares.server = nil, nil
}

// serveShared answers a link's page (/c/TOKEN), and its version
// (/c/TOKEN/version), which the page asks to reload when it changes.
func (s *Server) serveShared(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex")
	rest, ok := strings.CutPrefix(r.URL.Path, "/c/")
	token, sub, _ := strings.Cut(rest, "/")
	if !ok || r.Method != http.MethodGet || len(token) != 32 {
		http.NotFound(w, r)
		return
	}
	s.shares.mu.Lock()
	i := slices.IndexFunc(s.shares.file.Shares, func(x proto.CanvasShare) bool { return x.Token == token })
	var id string
	if i >= 0 {
		id = s.shares.file.Shares[i].Canvas
	}
	s.shares.mu.Unlock()
	if id == "" {
		http.NotFound(w, r)
		return
	}
	c, err := s.canvasFor(id).Get(id)
	if err != nil {
		http.Error(w, "this canvas is gone", http.StatusGone)
		return
	}
	version := strconv.Itoa(c.Version) + "-" + strconv.FormatInt(c.Updated.UnixNano(), 10)
	switch sub {
	case "version":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(version))
	case "":
		assets := func(name string) ([]byte, error) { return s.canvasFor(c.ID).Asset(c.ID, name) }
		page := canvas.Standalone(c, assets)
		reload := `<script>(()=>{const v=` + strconv.Quote(version) + `;setInterval(async()=>{try{const r=await fetch(location.pathname.replace(/\/$/,'')+'/version',{cache:'no-store'});if(r.ok&&await r.text()!==v)location.reload()}catch(e){}},3000)})()</script>`
		if j := strings.LastIndex(page, "</body>"); j >= 0 {
			page = page[:j] + reload + page[j:]
		} else {
			page += reload
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	default:
		http.NotFound(w, r)
	}
}

// tailnetIP is this host's Tailscale address (100.64.0.0/10), "" off one.
func tailnetIP() string {
	_, cgnat, _ := net.ParseCIDR("100.64.0.0/10")
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && cgnat.Contains(n.IP) {
			return n.IP.String()
		}
	}
	return ""
}
