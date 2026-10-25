package intercept

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// maxBuffered bounds a body read whole to match a rule, transform it or
// pause it; past it the rest streams unchanged.
const maxBuffered = 32 << 20

// hopHeaders are the connection's own, not forwarded.
var hopHeaders = []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Trailer", "Transfer-Encoding", "Upgrade"}

func removeHop(h http.Header, keepUpgrade bool) {
	for _, f := range h.Values("Connection") {
		for _, name := range strings.Split(f, ",") {
			if n := strings.TrimSpace(name); n != "" && !(keepUpgrade && strings.EqualFold(n, "Upgrade")) {
				h.Del(n)
			}
		}
	}
	for _, n := range hopHeaders {
		if keepUpgrade && (n == "Upgrade" || n == "Connection") {
			continue
		}
		h.Del(n)
	}
	// gRPC needs "TE: trailers"; other TE values are the connection's.
	if te := h.Get("Te"); te != "" && !strings.EqualFold(strings.TrimSpace(te), "trailers") {
		h.Del("Te")
	}
}

// serveExchange handles one request that came through the proxy.
func (p *Proxy) serveExchange(w http.ResponseWriter, r *http.Request, info connInfo) {
	src := info.src
	if src.ID == "" {
		s, ok := p.auth(r.Header.Get("Proxy-Authorization"))
		if !ok {
			w.Header().Set("Proxy-Authenticate", `Basic realm="enowX"`)
			http.Error(w, "enowX's proxy needs its credentials", http.StatusProxyAuthRequired)
			return
		}
		src = s
	}
	u := *r.URL
	switch {
	case info.reverse:
		u.Scheme, u.Host = info.scheme, info.target
	case !u.IsAbs():
		u.Scheme = info.scheme
		u.Host = r.Host
		if u.Host == "" {
			u.Host = info.target
		}
	}
	u.Host = trimDefaultPort(u.Scheme, u.Host)
	ws := isWebSocket(r)
	removeHop(r.Header, ws)
	r.Header.Del("Proxy-Authorization")
	p.exchange(w, r, &u, src, info, ws)
}

// exchange runs a request through the rules and on upstream, recording
// it. w is the client's; r's body is the client's request body.
func (p *Proxy) exchange(w http.ResponseWriter, r *http.Request, u *url.URL, src Source, info connInfo, ws bool) *Exchange {
	kind := proto.TrafficHTTP
	switch {
	case ws:
		kind = proto.TrafficWS
	case kindOf(r.Header.Get("Content-Type")) == "grpc":
		kind = proto.TrafficGRPC
	}
	ex := p.storeFor(src).begin(proto.TrafficSummary{
		Source: src.ID, Session: src.Session, Method: r.Method, URL: u.String(), Host: u.Hostname(),
		Path: u.RequestURI(), Proto: r.Proto, Kind: kind,
	})
	start := time.Now()
	ex.update(func(ex *Exchange) {
		ex.reqHead = r.Header.Clone()
		ex.req.ctype, ex.req.encoding = r.Header.Get("Content-Type"), r.Header.Get("Content-Encoding")
		ex.tls = info.tls
	})
	var rules *Rules
	if p.RulesFor != nil && !src.Quiet && !src.NoRules {
		rules = p.RulesFor(src)
	}

	// The body is read first when a rule could need it.
	var reqBody []byte
	buffered := false
	if r.Body != nil && r.Body != http.NoBody && rules.needsBody() {
		data, err := io.ReadAll(io.LimitReader(r.Body, maxBuffered+1))
		if err != nil {
			p.failed(ex, start, err)
			http.Error(w, "enowX: reading the request: "+err.Error(), http.StatusBadGateway)
			return ex
		}
		if len(data) > maxBuffered {
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(data), r.Body))
		} else {
			reqBody, buffered = data, true
		}
	}
	decodedReq := reqBody
	if buffered {
		decodedReq, _ = decode(reqBody, r.Header.Get("Content-Encoding"))
	}
	rule, matched := rules.match(request{method: r.Method, url: u, header: r.Header, body: decodedReq, source: src.ID, session: src.Session, ws: ws})
	a := rule.Action
	if matched {
		label := rule.Name
		if label == "" {
			label = rule.ID
		}
		ex.update(func(ex *Exchange) { ex.sum.Rule = label })
		if a.DelayMs > 0 {
			select {
			case <-time.After(time.Duration(a.DelayMs) * time.Millisecond):
			case <-r.Context().Done():
				p.failed(ex, start, r.Context().Err())
				return ex
			}
		}
	}

	switch a.Kind {
	case proto.ActionFail:
		p.fail(w, r, a, ex, start)
		return ex
	case proto.ActionMock:
		if ws {
			p.mockWebSocket(w, r, a, ex, start)
			return ex
		}
		p.mock(w, rules, a, ex, start, buffered, reqBody, r)
		return ex
	case proto.ActionForward:
		t, _ := url.Parse(a.Target)
		u.Scheme, u.Host = t.Scheme, t.Host
		if t.Path != "" && t.Path != "/" {
			u.Path = strings.TrimSuffix(t.Path, "/") + u.Path
		}
		if !a.KeepHost {
			r.Host = u.Host
		}
		ex.update(func(ex *Exchange) { ex.sum.URL, ex.sum.Host = u.String(), u.Hostname() })
	case proto.ActionTransform:
		if t := a.Request; t != nil {
			transformHeaders(r.Header, t)
			if t.URL != "" {
				if nu, err := u.Parse(t.URL); err == nil {
					*u = *nu
				}
			}
			if buffered && (t.Body != nil || len(t.Replace) > 0 || len(t.JSON) > 0) {
				reqBody = rules.transformBody(rule.ID, decodedReq, t)
				r.Header.Del("Content-Encoding")
			}
		}
	case proto.ActionBreakpoint:
		if a.Break != "response" {
			res := p.holdExchange(r.Context(), ex, proto.TrafficPaused{
				ID: ex.ID(), Stage: "request", Source: src.ID, Session: src.Session,
				Method: r.Method, URL: u.String(), Headers: headerList(r.Header), Body: decodedReq,
			})
			if res.drop {
				p.failed(ex, start, errDropped)
				dropConn(w)
				return ex
			}
			if e := res.edit; e != nil {
				if e.Method != "" {
					r.Method = e.Method
				}
				if e.URL != "" {
					if nu, err := url.Parse(e.URL); err == nil && nu.IsAbs() {
						*u = *nu
						r.Host = nu.Host
					}
				}
				if e.Headers != nil {
					r.Header = headerMap(e.Headers)
				}
				if e.Body != nil {
					reqBody, buffered = *e.Body, true
					r.Header.Del("Content-Encoding")
				}
			}
			ex.update(func(ex *Exchange) {
				ex.sum.Method, ex.sum.URL, ex.sum.Host, ex.sum.Path = r.Method, u.String(), u.Hostname(), u.RequestURI()
				ex.reqHead = r.Header.Clone()
			})
		}
	}

	if ws {
		host := u.Host
		if info.reverse && r.Host != "" {
			host = r.Host
		}
		p.relayWebSocket(w, r, u, host, ex, start)
		return ex
	}

	// The request upstream.
	var body io.Reader
	switch {
	case buffered:
		ex.keep(&ex.req, reqBody)
		body = bytes.NewReader(reqBody)
	case r.Body != nil && r.Body != http.NoBody:
		body = &recordingReader{r: r.Body, ex: ex, b: &ex.req, stream: streamParser(ex, true, r.Header.Get("Content-Type"))}
	}
	out, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), body)
	if err != nil {
		p.failed(ex, start, err)
		http.Error(w, "enowX: "+err.Error(), http.StatusBadGateway)
		return ex
	}
	out.Header = r.Header.Clone()
	out.Host = r.Host
	if out.Host == "" || (!info.reverse && !strings.EqualFold(stripPort(out.Host), u.Hostname())) {
		out.Host = u.Host
	}
	if buffered {
		out.ContentLength = int64(len(reqBody))
		out.Header.Set("Content-Length", strconv.Itoa(len(reqBody)))
	} else {
		out.ContentLength = r.ContentLength
	}
	if out.ContentLength == 0 && body == nil {
		out.Body = http.NoBody
	}
	if len(r.Trailer) > 0 {
		out.Trailer = r.Trailer
	}
	// The trace's callbacks run on the transport's goroutines.
	at := func(set func(t *proto.TrafficTimings, ms int64)) {
		ms := time.Since(start).Milliseconds()
		ex.mu.Lock()
		set(&ex.timings, ms)
		ex.mu.Unlock()
	}
	trace := &httptrace.ClientTrace{
		ConnectDone:          func(string, string, error) { at(func(t *proto.TrafficTimings, ms int64) { t.Connect = ms }) },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { at(func(t *proto.TrafficTimings, ms int64) { t.TLS = ms }) },
		WroteRequest:         func(httptrace.WroteRequestInfo) { at(func(t *proto.TrafficTimings, ms int64) { t.Sent = ms }) },
		GotFirstResponseByte: func() { at(func(t *proto.TrafficTimings, ms int64) { t.FirstByte = ms }) },
		GotConn: func(ci httptrace.GotConnInfo) {
			if a := ci.Conn.RemoteAddr(); a != nil {
				ex.update(func(ex *Exchange) { ex.remote = a.String() })
			}
		},
	}
	out = out.WithContext(httptrace.WithClientTrace(out.Context(), trace))
	resp, err := p.transport(u, info.h2c).RoundTrip(out)
	if err != nil {
		p.failed(ex, start, err)
		if r.Context().Err() == nil {
			http.Error(w, "enowX couldn't reach "+u.Host+": "+err.Error(), http.StatusBadGateway)
		}
		return ex
	}
	defer resp.Body.Close()
	removeHop(resp.Header, false)
	ex.update(func(ex *Exchange) {
		ex.sum.Status, ex.sum.Proto = resp.StatusCode, protoName(r.Proto, resp.Proto)
		ex.resHead = resp.Header.Clone()
		ex.res.ctype, ex.res.encoding = resp.Header.Get("Content-Type"), resp.Header.Get("Content-Encoding")
		ex.sum.Type = kindOf(ex.res.ctype)
		if ex.sum.Type == "sse" {
			ex.sum.Kind = proto.TrafficSSE
		}
	})

	// A response to change or pause is read whole first.
	resT := a.Kind == proto.ActionTransform && a.Response != nil
	resB := a.Kind == proto.ActionBreakpoint && (a.Break == "response" || a.Break == "both")
	if resT || resB {
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxBuffered))
		if err != nil {
			p.failed(ex, start, err)
			http.Error(w, "enowX: reading the response: "+err.Error(), http.StatusBadGateway)
			return ex
		}
		status, header := resp.StatusCode, resp.Header
		decoded, ok := decode(data, header.Get("Content-Encoding"))
		changed := false
		if resT {
			t := a.Response
			transformHeaders(header, t)
			if t.Status != 0 {
				status = t.Status
			}
			if ok && (t.Body != nil || len(t.Replace) > 0 || len(t.JSON) > 0) {
				decoded, changed = rules.transformBody(rule.ID, decoded, t), true
			}
		}
		if resB {
			res := p.holdExchange(r.Context(), ex, proto.TrafficPaused{
				ID: ex.ID(), Stage: "response", Source: src.ID, Session: src.Session,
				Method: r.Method, URL: u.String(), Status: status, Headers: headerList(header), Body: decoded,
			})
			if res.drop {
				p.failed(ex, start, errDropped)
				dropConn(w)
				return ex
			}
			if e := res.edit; e != nil {
				if e.Status != 0 {
					status = e.Status
				}
				if e.Headers != nil {
					header = headerMap(e.Headers)
				}
				if e.Body != nil {
					decoded, changed = *e.Body, true
				}
			}
		}
		if changed {
			data = decoded
			header.Del("Content-Encoding")
		}
		header.Set("Content-Length", strconv.Itoa(len(data)))
		copyHeader(w.Header(), header)
		w.WriteHeader(status)
		ex.keep(&ex.res, data)
		writeThrottled(w, data, a.Throttle)
		ex.update(func(ex *Exchange) {
			ex.sum.Status = status
			ex.resHead = header.Clone()
			ex.res.encoding = header.Get("Content-Encoding")
		})
		p.done(ex, start)
		return ex
	}

	// Streamed as it comes, flushed at once for streams.
	copyHeader(w.Header(), resp.Header)
	for k := range resp.Trailer {
		w.Header().Add("Trailer", k)
	}
	w.WriteHeader(resp.StatusCode)
	parser := streamParser(ex, false, resp.Header.Get("Content-Type"))
	flush := parser != nil || resp.ContentLength < 0
	buf := make([]byte, 32<<10)
	var tw io.Writer = w
	if a.Throttle > 0 {
		tw = &throttled{w: w, rate: a.Throttle}
	}
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			ex.keep(&ex.res, chunk)
			if parser != nil {
				parser(chunk)
			}
			if _, werr := tw.Write(chunk); werr != nil {
				p.failed(ex, start, werr)
				return ex
			}
			if flush {
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			p.failed(ex, start, err)
			return ex
		}
	}
	for k, vs := range resp.Trailer {
		for _, v := range vs {
			w.Header().Add(http.TrailerPrefix+k, v)
		}
	}
	ex.update(func(ex *Exchange) { ex.trailers = resp.Trailer.Clone() })
	p.done(ex, start)
	return ex
}

// mock answers a request from a rule, without going upstream.
func (p *Proxy) mock(w http.ResponseWriter, rules *Rules, a proto.TrafficAction, ex *Exchange, start time.Time, buffered bool, reqBody []byte, r *http.Request) {
	if buffered {
		ex.keep(&ex.req, reqBody)
	} else if r.Body != nil {
		data, _ := io.ReadAll(io.LimitReader(r.Body, ex.store.MaxBody))
		ex.keep(&ex.req, data)
	}
	data, err := rules.mockBody(a)
	if err != nil {
		p.failed(ex, start, err)
		http.Error(w, "enowX's mock: "+err.Error(), http.StatusInternalServerError)
		return
	}
	status := a.Status
	if status == 0 {
		status = http.StatusOK
	}
	h := headerMap(a.Headers)
	if h.Get("Content-Type") == "" && len(data) > 0 {
		h.Set("Content-Type", http.DetectContentType(data))
		if json := bytes.TrimSpace(data); len(json) > 0 && (json[0] == '{' || json[0] == '[') {
			h.Set("Content-Type", "application/json")
		}
	}
	h.Set("Content-Length", strconv.Itoa(len(data)))
	copyHeader(w.Header(), h)
	w.WriteHeader(status)
	writeThrottled(w, data, a.Throttle)
	ex.keep(&ex.res, data)
	ex.update(func(ex *Exchange) {
		ex.sum.Status, ex.sum.Mocked = status, true
		ex.resHead = h
		ex.res.ctype = h.Get("Content-Type")
		ex.sum.Type = kindOf(ex.res.ctype)
	})
	p.done(ex, start)
}

// fail fails a request as a rule says: closing or resetting the
// connection, never answering, or answering 503.
func (p *Proxy) fail(w http.ResponseWriter, r *http.Request, a proto.TrafficAction, ex *Exchange, start time.Time) {
	switch a.Fail {
	case "timeout":
		<-r.Context().Done()
		p.failed(ex, start, fmt.Errorf("timed out on purpose (a rule)"))
	case "close", "reset":
		p.failed(ex, start, fmt.Errorf("connection %sed on purpose (a rule)", strings.TrimSuffix(a.Fail, "e")))
		if hj, ok := w.(http.Hijacker); ok {
			if c, _, err := hj.Hijack(); err == nil {
				if tc, ok := underlyingTCP(c); ok && a.Fail == "reset" {
					tc.SetLinger(0)
				}
				c.Close()
				return
			}
		}
		panic(http.ErrAbortHandler) // HTTP/2: resets the stream
	default:
		status := a.Status
		if status == 0 {
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		io.WriteString(w, http.StatusText(status)+"\n")
		ex.update(func(ex *Exchange) { ex.sum.Status, ex.sum.Mocked = status, true })
		p.done(ex, start)
	}
}

func underlyingTCP(c net.Conn) (*net.TCPConn, bool) {
	for {
		switch t := c.(type) {
		case *net.TCPConn:
			return t, true
		case *tls.Conn:
			c = t.NetConn()
		case *peekedConn:
			c = t.Conn
		default:
			return nil, false
		}
	}
}

func dropConn(w http.ResponseWriter) {
	if hj, ok := w.(http.Hijacker); ok {
		if c, _, err := hj.Hijack(); err == nil {
			c.Close()
			return
		}
	}
	panic(http.ErrAbortHandler)
}

func (p *Proxy) done(ex *Exchange, start time.Time) {
	ex.update(func(ex *Exchange) {
		ex.sum.State = proto.TrafficDone
		ex.sum.Duration = time.Since(start).Milliseconds()
		ex.timings.Done = ex.sum.Duration
		ex.sum.ReqSize, ex.sum.ResSize = ex.req.size, ex.res.size
	})
}

func (p *Proxy) failed(ex *Exchange, start time.Time, err error) {
	ex.update(func(ex *Exchange) {
		ex.sum.State, ex.sum.Error = proto.TrafficFailed, err.Error()
		ex.sum.Duration = time.Since(start).Milliseconds()
		ex.sum.ReqSize, ex.sum.ResSize = ex.req.size, ex.res.size
	})
}

// transport is the round tripper for u: checking certificates or not,
// and HTTP/2 with prior knowledge for h2c.
func (p *Proxy) transport(u *url.URL, h2c bool) http.RoundTripper {
	t := &p.transports
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.strict == nil {
		t.strict, t.insecure = p.newTransport(false, nil), p.newTransport(true, nil)
		t.plain = p.newTransport(false, nil)
		var protos http.Protocols
		protos.SetUnencryptedHTTP2(true)
		t.plain.Protocols = &protos
	}
	if p.ClientCert != nil {
		if cert := p.ClientCert(u.Hostname()); cert != nil {
			// A host asking for a certificate gets a transport of its own.
			if t.byHost == nil {
				t.byHost = map[string]*http.Transport{}
			}
			tr, ok := t.byHost[u.Hostname()]
			if !ok {
				tr = p.newTransport(p.Insecure != nil && p.Insecure(u.Hostname()), cert)
				t.byHost[u.Hostname()] = tr
			}
			return tr
		}
	}
	switch {
	case h2c && u.Scheme == "http":
		return t.plain
	case p.Insecure != nil && p.Insecure(u.Hostname()):
		return t.insecure
	}
	return t.strict
}

// recordingReader records a request body as it streams upstream.
type recordingReader struct {
	r      io.ReadCloser
	ex     *Exchange
	b      *body
	stream func([]byte)
}

func (r *recordingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.ex.keep(r.b, p[:n])
		if r.stream != nil {
			r.stream(p[:n])
		}
	}
	return n, err
}

func (r *recordingReader) Close() error { return r.r.Close() }

// throttled writes at most rate bytes a second.
type throttled struct {
	w    io.Writer
	rate int
}

func (t *throttled) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n := min(len(p), max(t.rate/10, 1))
		m, err := t.w.Write(p[:n])
		written += m
		if err != nil {
			return written, err
		}
		if f, ok := t.w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(time.Duration(float64(n) / float64(t.rate) * float64(time.Second)))
		p = p[n:]
	}
	return written, nil
}

func writeThrottled(w io.Writer, data []byte, rate int) {
	if rate > 0 {
		(&throttled{w: w, rate: rate}).Write(data)
		return
	}
	w.Write(data)
}

func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		dst[k] = append([]string(nil), vs...)
	}
}

func headerMap(hs []proto.Header) http.Header {
	h := http.Header{}
	for _, x := range hs {
		h.Add(x.Name, x.Value)
	}
	return h
}

func isWebSocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && headerHasToken(r.Header, "Connection", "upgrade")
}

func headerHasToken(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, t := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

func trimDefaultPort(scheme, host string) string {
	if h, port, err := net.SplitHostPort(host); err == nil {
		if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
			if strings.Contains(h, ":") {
				return "[" + h + "]"
			}
			return h
		}
	}
	return host
}

func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// protoName is the HTTP version an exchange had: the client's, and the
// server's when it differs.
func protoName(client, server string) string {
	if server == "" || server == client {
		return client
	}
	return client + " → " + server
}

// newTransport is a transport upstream: checking certificates or not,
// showing cert to servers that ask, through the upstream proxy, with
// names resolved as Resolve says.
func (p *Proxy) newTransport(insecure bool, cert *tls.Certificate) *http.Transport {
	cfg := &tls.Config{InsecureSkipVerify: insecure}
	if cert != nil {
		cfg.Certificates = []tls.Certificate{*cert}
	}
	var d net.Dialer
	tr := &http.Transport{
		Proxy:               nil, // never the daemon's own environment, which may point here
		ForceAttemptHTTP2:   true,
		DisableCompression:  true,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig:     cfg,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return d.DialContext(ctx, network, p.resolved(addr))
		},
	}
	if p.Upstream != nil {
		tr.Proxy = http.ProxyURL(p.Upstream)
	}
	return tr
}
