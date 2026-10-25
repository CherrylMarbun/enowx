package intercept

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// Source is what traffic comes from: a session's tabs, a browser, a
// device. Its id and token are the proxy's credentials for it.
type Source struct {
	ID      string `json:"id"`
	Token   string `json:"token"`
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Session string `json:"session,omitempty"`
	Project string `json:"project,omitempty"`
	// Quiet passes its traffic on without recording it or applying rules:
	// a source detached whose programs still point here.
	Quiet bool `json:"quiet,omitempty"`
	// NoRules records its traffic without rules (a request sent around
	// them).
	NoRules bool `json:"-"`
}

// Proxy is a host's interceptor: an HTTP and SOCKS5 proxy on loopback. Its
// fields are set before Listen; sources, pauses and the listener belong
// to its mutex.
type Proxy struct {
	CA    *CA
	Store *Store
	// RulesFor is the rules for traffic of src (its project's), nil for
	// none.
	RulesFor func(src Source) *Rules
	// Upstream is a proxy the interceptor sends through, nil for none.
	Upstream *url.URL
	// Insecure reports whether a host's certificate isn't checked
	// upstream (self-signed development servers).
	Insecure func(host string) bool
	// OnPaused learns of the paused exchanges whenever they change.
	OnPaused func([]proto.TrafficPaused)

	mu      sync.Mutex
	sources map[string]Source
	ports   map[string]net.Listener // sources' own ports, by source id
	pauses  map[int64]*pause
	ln      net.Listener
	conns   map[net.Conn]struct{}

	transports struct {
		mu                      sync.Mutex
		strict, insecure, plain *http.Transport
	}
}

// connInfo is what a client connection told the proxy: who it is, and
// for a tunnel where to and with which TLS.
type connInfo struct {
	src    Source
	scheme string // inside a tunnel: http or https
	target string // the tunnel's host:port
	tls    *proto.TrafficTLS
	h2c    bool
	// reverse is a reverse proxy's connection: requests go to target
	// whatever their Host says, which stays as the browser sent it.
	reverse bool
}

// AddSource registers a source and returns it with its credentials.
func (p *Proxy) AddSource(kind, label, session, project string) Source {
	s := Source{ID: kind + "-" + randomHex(4), Token: randomHex(16), Kind: kind, Label: label, Session: session, Project: project}
	p.mu.Lock()
	if p.sources == nil {
		p.sources = map[string]Source{}
	}
	p.sources[s.ID] = s
	p.mu.Unlock()
	return s
}

// RestoreSource registers a source kept from before, with its credentials.
func (p *Proxy) RestoreSource(s Source) {
	p.mu.Lock()
	if p.sources == nil {
		p.sources = map[string]Source{}
	}
	p.sources[s.ID] = s
	p.mu.Unlock()
}

// RemoveSource forgets a source: its credentials and its port stop
// working.
func (p *Proxy) RemoveSource(id string) {
	p.mu.Lock()
	delete(p.sources, id)
	ln := p.ports[id]
	delete(p.ports, id)
	p.mu.Unlock()
	if ln != nil {
		ln.Close()
	}
}

// SetQuiet makes a source's traffic pass unrecorded, or recorded again.
func (p *Proxy) SetQuiet(id string, quiet bool) {
	p.mu.Lock()
	if s, ok := p.sources[id]; ok {
		s.Quiet = quiet
		p.sources[id] = s
	}
	p.mu.Unlock()
}

// SourcePort is a loopback port of src's own, where clients that can't
// send credentials (Android's proxy setting, Chromium, Java) come
// without them; their traffic is src's. It is opened the first time, on
// port when free (the one it had before), else on any.
func (p *Proxy) SourcePort(src Source, port int) (int, error) {
	p.mu.Lock()
	if ln, ok := p.ports[src.ID]; ok {
		p.mu.Unlock()
		return ln.Addr().(*net.TCPAddr).Port, nil
	}
	p.mu.Unlock()
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil && port != 0 {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		return 0, fmt.Errorf("intercept: %w", err)
	}
	p.mu.Lock()
	if p.ports == nil {
		p.ports = map[string]net.Listener{}
	}
	p.ports[src.ID] = ln
	p.mu.Unlock()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go p.handleConnAs(c, &src.ID)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// SourceByID returns a source registered.
func (p *Proxy) SourceByID(id string) (Source, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.sources[id]
	return s, ok
}

// URL is the proxy's address with src's credentials.
func (p *Proxy) URL(src Source) string {
	return "http://" + src.ID + ":" + src.Token + "@" + p.Addr()
}

// Addr is the address the proxy listens on, "" before Listen.
func (p *Proxy) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ln == nil {
		return ""
	}
	return p.ln.Addr().String()
}

// Port is the port the proxy listens on, 0 before Listen.
func (p *Proxy) Port() int {
	ln := p.listener()
	if ln == nil {
		return 0
	}
	if t, ok := ln.Addr().(*net.TCPAddr); ok {
		return t.Port
	}
	return 0
}

func (p *Proxy) listener() net.Listener {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ln
}

// Listen listens on addr (a loopback address; "127.0.0.1:0" picks a
// port) and serves until Close.
func (p *Proxy) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("intercept: %w", err)
	}
	p.mu.Lock()
	p.ln = ln
	p.conns = map[net.Conn]struct{}{}
	p.mu.Unlock()
	go p.serve(ln)
	return nil
}

// Close stops listening and closes the connections the proxy holds.
func (p *Proxy) Close() error {
	p.mu.Lock()
	ln := p.ln
	p.ln = nil
	for _, l := range p.ports {
		l.Close()
	}
	p.ports = nil
	conns := p.conns
	p.conns = nil
	pauses := p.pauses
	p.pauses = nil
	p.mu.Unlock()
	for _, ps := range pauses {
		ps.resolve(resume{drop: true})
	}
	for c := range conns {
		c.Close()
	}
	if ln != nil {
		return ln.Close()
	}
	return nil
}

func (p *Proxy) serve(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go p.handleConnAs(c, nil)
	}
}

func (p *Proxy) track(c net.Conn, on bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conns == nil {
		return
	}
	if on {
		p.conns[c] = struct{}{}
	} else {
		delete(p.conns, c)
	}
}

// handleConnAs serves a client connection: SOCKS5 (its first byte is 5),
// a CONNECT tunnel, or plain HTTP proxy requests. On a source's own port
// as is that source, without credentials.
func (p *Proxy) handleConnAs(c net.Conn, asID *string) {
	var as *Source
	if asID != nil {
		s, ok := p.SourceByID(*asID)
		if !ok {
			c.Close()
			return
		}
		as = &s
	}
	p.track(c, true)
	defer p.track(c, false)
	br := bufio.NewReader(c)
	c.SetReadDeadline(time.Now().Add(30 * time.Second))
	first, err := br.Peek(1)
	if err != nil {
		c.Close()
		return
	}
	if first[0] == 5 {
		p.socks(&peekedConn{Conn: c, r: br}, as)
		return
	}
	head, _ := br.Peek(8)
	if strings.HasPrefix(string(head), "CONNECT ") {
		req, err := http.ReadRequest(br)
		if err != nil {
			c.Close()
			return
		}
		c.SetReadDeadline(time.Time{})
		src, ok := p.auth(req.Header.Get("Proxy-Authorization"))
		if as != nil {
			src, ok = *as, true
		}
		if !ok {
			io.WriteString(c, "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"enowX\"\r\nContent-Length: 0\r\n\r\n")
			c.Close()
			return
		}
		io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
		p.tunnel(&peekedConn{Conn: c, r: br}, req.Host, src)
		return
	}
	c.SetReadDeadline(time.Time{})
	info := connInfo{scheme: "http"}
	if as != nil {
		info.src = *as
	}
	p.serveHTTP1(&peekedConn{Conn: c, r: br}, info)
}

// auth checks a Proxy-Authorization header (Basic, source:token).
func (p *Proxy) auth(header string) (Source, bool) {
	scheme, cred, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return Source{}, false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cred))
	if err != nil {
		return Source{}, false
	}
	user, pass, _ := strings.Cut(string(raw), ":")
	return p.check(user, pass)
}

func (p *Proxy) check(user, pass string) (Source, bool) {
	p.mu.Lock()
	s, ok := p.sources[user]
	p.mu.Unlock()
	if !ok || subtle.ConstantTimeCompare([]byte(s.Token), []byte(pass)) != 1 {
		return Source{}, false
	}
	return s, true
}

// tunnel serves what a client sends through a tunnel to target: TLS,
// terminated with the CA's certificate; HTTP/2 with prior knowledge;
// plain HTTP; anything else passed through as it is.
func (p *Proxy) tunnel(c *peekedConn, target string, src Source) {
	if !strings.Contains(target, ":") {
		target += ":443"
	}
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	first, err := c.r.Peek(1)
	c.SetReadDeadline(time.Time{})
	if err != nil {
		// The client waits for the server to speak first (SMTP, a
		// database): pass it through.
		p.passThrough(c, target, src, nil)
		return
	}
	switch {
	case first[0] == 0x16: // a TLS handshake
		host, _, _ := net.SplitHostPort(target)
		cfg := &tls.Config{
			NextProtos: []string{"h2", "http/1.1"},
			GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				name := hello.ServerName
				if name == "" {
					name = host
				}
				return p.CA.Leaf(name)
			},
		}
		tc := tls.Server(c, cfg)
		if err := tc.Handshake(); err != nil {
			p.recordFailedTLS(src, target, err)
			tc.Close()
			return
		}
		st := tc.ConnectionState()
		info := connInfo{src: src, scheme: "https", target: target, tls: &proto.TrafficTLS{
			Version: tls.VersionName(st.Version), Cipher: tls.CipherSuiteName(st.CipherSuite),
			ServerName: st.ServerName, ALPN: st.NegotiatedProtocol,
		}}
		if st.NegotiatedProtocol == "h2" {
			p.serveHTTP2(tc, info)
			return
		}
		p.serveHTTP1(tc, info)
	case looksLikeH2(c.r):
		p.serveHTTP2(c, connInfo{src: src, scheme: "http", target: target, h2c: true})
	case first[0] >= 'A' && first[0] <= 'Z':
		p.serveHTTP1(c, connInfo{src: src, scheme: "http", target: target})
	default:
		p.passThrough(c, target, src, nil)
	}
}

// h2Preface starts an HTTP/2 connection with prior knowledge.
const h2Preface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

func looksLikeH2(r *bufio.Reader) bool {
	b, err := r.Peek(len(h2Preface))
	return err == nil && string(b) == h2Preface
}

// recordFailedTLS records a TLS handshake the client gave up, which is
// how an app that doesn't trust the CA shows.
func (p *Proxy) recordFailedTLS(src Source, target string, err error) {
	host, _, _ := net.SplitHostPort(target)
	msg := err.Error()
	if strings.Contains(msg, "unknown certificate") || strings.Contains(msg, "bad certificate") || strings.Contains(msg, "certificate unknown") {
		msg = "the client refused enowX's certificate: it doesn't trust the CA (" + msg + ")"
	}
	ex := p.storeFor(src).begin(proto.TrafficSummary{Source: src.ID, Session: src.Session, Method: "CONNECT", URL: "https://" + target, Host: host, Kind: proto.TrafficTunnel})
	ex.update(func(ex *Exchange) { ex.sum.State, ex.sum.Error = proto.TrafficFailed, msg })
}

// passThrough copies bytes both ways between the client and target,
// recorded as a tunnel with their sizes.
func (p *Proxy) passThrough(c net.Conn, target string, src Source, _ []byte) {
	defer c.Close()
	host, _, _ := net.SplitHostPort(target)
	ex := p.storeFor(src).begin(proto.TrafficSummary{Source: src.ID, Session: src.Session, Method: "CONNECT", URL: "tcp://" + target, Host: host, Kind: proto.TrafficTunnel})
	start := time.Now()
	up, err := p.dialUpstream(context.Background(), target)
	if err != nil {
		ex.update(func(ex *Exchange) { ex.sum.State, ex.sum.Error = proto.TrafficFailed, err.Error() })
		return
	}
	defer up.Close()
	var sent, got int64
	done := make(chan struct{})
	go func() {
		sent, _ = io.Copy(up, c)
		if cw, ok := up.(interface{ CloseWrite() error }); ok {
			cw.CloseWrite()
		}
		close(done)
	}()
	got, _ = io.Copy(c, up)
	c.Close()
	<-done
	ex.update(func(ex *Exchange) {
		ex.sum.State, ex.sum.ReqSize, ex.sum.ResSize = proto.TrafficDone, sent, got
		ex.sum.Duration = time.Since(start).Milliseconds()
		ex.remote = up.RemoteAddr().String()
	})
}

// dialUpstream connects to target, through the upstream proxy if one is
// set (CONNECT).
func (p *Proxy) dialUpstream(ctx context.Context, target string) (net.Conn, error) {
	var d net.Dialer
	if p.Upstream == nil {
		return d.DialContext(ctx, "tcp", target)
	}
	c, err := d.DialContext(ctx, "tcp", p.Upstream.Host)
	if err != nil {
		return nil, err
	}
	req := &http.Request{Method: "CONNECT", URL: &url.URL{Opaque: target}, Host: target, Header: http.Header{}}
	if u := p.Upstream.User; u != nil {
		pass, _ := u.Password()
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u.Username()+":"+pass)))
	}
	if err := req.Write(c); err != nil {
		c.Close()
		return nil, err
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		c.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		c.Close()
		return nil, fmt.Errorf("the upstream proxy answered %s", resp.Status)
	}
	return &peekedConn{Conn: c, r: br}, nil
}

// serveHTTP1 serves the requests on c until it closes: HTTP/1.x, HTTP/2
// a TLS handshake agreed on, or HTTP/2 with prior knowledge.
func (p *Proxy) serveHTTP1(c net.Conn, info connInfo) {
	ln := newOneConnListener(c)
	var protos http.Protocols
	protos.SetHTTP1(true)
	protos.SetHTTP2(true)
	protos.SetUnencryptedHTTP2(true)
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { p.serveExchange(w, r, info) }),
		ReadHeaderTimeout: time.Minute,
		ErrorLog:          quietLog,
		Protocols:         &protos,
		ConnState: func(_ net.Conn, st http.ConnState) {
			if st == http.StateClosed || st == http.StateHijacked {
				ln.Close()
			}
		},
	}
	srv.Serve(ln)
}

// serveHTTP2 serves HTTP/2 on c, as negotiated or with prior knowledge.
func (p *Proxy) serveHTTP2(c net.Conn, info connInfo) { p.serveHTTP1(c, info) }

// socks serves a SOCKS5 client: username and password authentication (a
// source's id and token), then CONNECT.
func (p *Proxy) socks(c *peekedConn, as *Source) {
	fail := func() { c.Close() }
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(c, hdr); err != nil {
		fail()
		return
	}
	methods := make([]byte, hdr[1])
	if _, err := io.ReadFull(c, methods); err != nil {
		fail()
		return
	}
	hasPass, hasNone := false, false
	for _, m := range methods {
		hasPass, hasNone = hasPass || m == 2, hasNone || m == 0
	}
	var src Source
	switch {
	case as != nil && hasNone:
		c.Write([]byte{5, 0})
		src = *as
	case hasPass:
		c.Write([]byte{5, 2})
		// RFC 1929: version, user, password.
		var ver [1]byte
		if _, err := io.ReadFull(c, ver[:]); err != nil {
			fail()
			return
		}
		user, err1 := readPascal(c)
		pass, err2 := readPascal(c)
		if err1 != nil || err2 != nil {
			fail()
			return
		}
		s, ok := p.check(user, pass)
		if as != nil {
			s, ok = *as, true
		}
		if !ok {
			c.Write([]byte{1, 1})
			fail()
			return
		}
		c.Write([]byte{1, 0})
		src = s
	default:
		c.Write([]byte{5, 0xff})
		fail()
		return
	}
	req := make([]byte, 4)
	if _, err := io.ReadFull(c, req); err != nil || req[1] != 1 {
		c.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		fail()
		return
	}
	var host string
	switch req[3] {
	case 1:
		b := make([]byte, 4)
		io.ReadFull(c, b)
		host = net.IP(b).String()
	case 3:
		name, err := readPascal(c)
		if err != nil {
			fail()
			return
		}
		host = name
	case 4:
		b := make([]byte, 16)
		io.ReadFull(c, b)
		host = net.IP(b).String()
	default:
		c.Write([]byte{5, 8, 0, 1, 0, 0, 0, 0, 0, 0})
		fail()
		return
	}
	var port [2]byte
	if _, err := io.ReadFull(c, port[:]); err != nil {
		fail()
		return
	}
	c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	p.tunnel(c, net.JoinHostPort(host, strconv.Itoa(int(port[0])<<8|int(port[1]))), src)
}

func readPascal(r io.Reader) (string, error) {
	var n [1]byte
	if _, err := io.ReadFull(r, n[:]); err != nil {
		return "", err
	}
	b := make([]byte, n[0])
	_, err := io.ReadFull(r, b)
	return string(b), err
}

// peekedConn is a connection whose first bytes were peeked through r.
type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// oneConnListener hands out one connection, then waits until closed.
type oneConnListener struct {
	c    net.Conn
	once sync.Once
	done chan struct{}
	took bool
	mu   sync.Mutex
}

func newOneConnListener(c net.Conn) *oneConnListener {
	return &oneConnListener{c: c, done: make(chan struct{})}
}

func (l *oneConnListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if !l.took {
		l.took = true
		l.mu.Unlock()
		return l.c, nil
	}
	l.mu.Unlock()
	<-l.done
	return nil, net.ErrClosed
}

func (l *oneConnListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *oneConnListener) Addr() net.Addr { return l.c.LocalAddr() }

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

var errDropped = errors.New("dropped at a breakpoint")

// quietLog drops the HTTP servers' own logging of clients' errors.
var quietLog = log.New(io.Discard, "", 0)

// discarded keeps nothing: quiet sources' exchanges go there.
var discarded = &Store{MaxExchanges: 0, MaxBytes: 0, MaxBody: 0, byID: map[int64]*Exchange{}}

// storeFor is where src's exchanges are recorded.
func (p *Proxy) storeFor(src Source) *Store {
	if src.Quiet {
		return discarded
	}
	return p.Store
}

// ResetTransports makes upstream connections anew, after the upstream
// proxy changed.
func (p *Proxy) ResetTransports() {
	t := &p.transports
	t.mu.Lock()
	for _, tr := range []*http.Transport{t.strict, t.insecure, t.plain} {
		if tr != nil {
			tr.CloseIdleConnections()
		}
	}
	t.strict, t.insecure, t.plain = nil, nil, nil
	t.mu.Unlock()
}

// Reverse listens on a loopback port of its own as a reverse proxy in
// front of target (a session's dev server, host:port): what a browser
// sends there goes to target, recorded as src's, rules applied. Close the
// listener to stop.
func (p *Proxy) Reverse(src Source, target string) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("intercept: %w", err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				p.track(c, true)
				defer p.track(c, false)
				p.serveHTTP1(c, connInfo{src: src, scheme: "http", target: target, reverse: true})
			}()
		}
	}()
	return ln, nil
}
