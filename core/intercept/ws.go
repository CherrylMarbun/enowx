package intercept

import (
	"bufio"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// WebSocket (RFC 6455): the upgrade goes upstream, then frames are copied
// both ways as they are, each recorded as a message, fragments joined.

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Opcodes.
const (
	wsCont   = 0
	wsText   = 1
	wsBinary = 2
	wsClose  = 8
	wsPing   = 9
	wsPong   = 10
)

var wsKinds = map[byte]string{wsText: "text", wsBinary: "binary", wsClose: "close", wsPing: "ping", wsPong: "pong"}

// relayWebSocket takes a WebSocket upgrade upstream and relays its frames.
func (p *Proxy) relayWebSocket(w http.ResponseWriter, r *http.Request, u *url.URL, host string, ex *Exchange, start time.Time) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		p.failed(ex, start, errors.New("WebSocket over HTTP/2 isn't relayed"))
		http.Error(w, "enowX relays WebSocket over HTTP/1.1", http.StatusHTTPVersionNotSupported)
		return
	}
	// Uncompressed frames can be read on the way.
	r.Header.Del("Sec-WebSocket-Extensions")
	port := u.Port()
	if port == "" {
		port = map[string]string{"https": "443", "wss": "443"}[u.Scheme]
		if port == "" {
			port = "80"
		}
	}
	target := net.JoinHostPort(u.Hostname(), port)
	up, err := p.dialUpstream(r.Context(), target)
	if err != nil {
		p.failed(ex, start, err)
		http.Error(w, "enowX couldn't reach "+target+": "+err.Error(), http.StatusBadGateway)
		return
	}
	defer up.Close()
	if u.Scheme == "https" || u.Scheme == "wss" {
		insecure := p.Insecure != nil && p.Insecure(u.Hostname())
		tc := tls.Client(up, &tls.Config{ServerName: u.Hostname(), NextProtos: []string{"http/1.1"}, InsecureSkipVerify: insecure})
		if err := tc.HandshakeContext(r.Context()); err != nil {
			p.failed(ex, start, err)
			http.Error(w, "enowX: TLS with "+target+": "+err.Error(), http.StatusBadGateway)
			return
		}
		up = tc
	}
	out := &http.Request{Method: r.Method, URL: &url.URL{Path: u.Path, RawPath: u.RawPath, RawQuery: u.RawQuery}, Host: host, Header: r.Header.Clone(), Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}
	out.Header.Set("Connection", "Upgrade")
	out.Header.Set("Upgrade", "websocket")
	if err := out.Write(up); err != nil {
		p.failed(ex, start, err)
		http.Error(w, "enowX: "+err.Error(), http.StatusBadGateway)
		return
	}
	upr := bufio.NewReader(up)
	resp, err := http.ReadResponse(upr, out)
	if err != nil {
		p.failed(ex, start, err)
		http.Error(w, "enowX: "+err.Error(), http.StatusBadGateway)
		return
	}
	ex.update(func(ex *Exchange) {
		ex.sum.Status = resp.StatusCode
		ex.resHead = resp.Header.Clone()
		ex.remote = up.RemoteAddr().String()
	})
	c, brw, err := hj.Hijack()
	if err != nil {
		p.failed(ex, start, err)
		return
	}
	defer c.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		resp.Write(c)
		p.done(ex, start)
		return
	}
	writeHead(c, resp.StatusCode, resp.Header)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		relayFrames(brw.Reader, up, ex, true)
		if cw, ok := up.(interface{ CloseWrite() error }); ok {
			cw.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		relayFrames(upr, c, ex, false)
		c.Close()
	}()
	wg.Wait()
	p.done(ex, start)
}

// mockWebSocket answers an upgrade itself: sends the rule's messages, and
// echoes the client's when asked.
func (p *Proxy) mockWebSocket(w http.ResponseWriter, r *http.Request, a proto.TrafficAction, ex *Exchange, start time.Time) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "enowX mocks WebSocket over HTTP/1.1", http.StatusHTTPVersionNotSupported)
		return
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	sum := sha1.Sum([]byte(key + wsGUID))
	h := http.Header{}
	h.Set("Upgrade", "websocket")
	h.Set("Connection", "Upgrade")
	h.Set("Sec-WebSocket-Accept", base64.StdEncoding.EncodeToString(sum[:]))
	c, brw, err := hj.Hijack()
	if err != nil {
		p.failed(ex, start, err)
		return
	}
	defer c.Close()
	writeHead(c, http.StatusSwitchingProtocols, h)
	ex.update(func(ex *Exchange) { ex.sum.Status, ex.sum.Mocked, ex.resHead = 101, true, h })
	var mu sync.Mutex
	send := func(op byte, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		ex.addMessage(proto.TrafficMessage{Time: time.Now(), Kind: wsKinds[op], Data: data, Size: int64(len(data)), Mocked: true})
		return writeFrame(c, op, data)
	}
	for _, m := range a.Messages {
		if send(wsText, []byte(m)) != nil {
			p.done(ex, start)
			return
		}
	}
	for {
		f, err := readFrame(brw.Reader)
		if err != nil {
			break
		}
		ex.addMessage(proto.TrafficMessage{Sent: true, Time: time.Now(), Kind: wsKinds[f.op], Data: f.payload, Size: int64(len(f.payload))})
		switch f.op {
		case wsClose:
			send(wsClose, f.payload)
			p.done(ex, start)
			return
		case wsPing:
			send(wsPong, f.payload)
		case wsText, wsBinary:
			if a.Echo {
				send(f.op, f.payload)
			}
		}
	}
	p.done(ex, start)
}

func writeHead(w io.Writer, status int, h http.Header) {
	var b strings.Builder
	fmt.Fprintf(&b, "HTTP/1.1 %d %s\r\n", status, http.StatusText(status))
	h.Write(&b)
	b.WriteString("\r\n")
	io.WriteString(w, b.String())
}

// frame is a WebSocket frame as read: its raw bytes, to copy on as they
// are, and its payload unmasked.
type frame struct {
	raw     []byte
	fin     bool
	op      byte
	payload []byte
}

// maxFrame bounds a frame read whole.
const maxFrame = 64 << 20

func readFrame(r *bufio.Reader) (frame, error) {
	var f frame
	h := make([]byte, 2, 14)
	if _, err := io.ReadFull(r, h); err != nil {
		return f, err
	}
	f.fin, f.op = h[0]&0x80 != 0, h[0]&0x0f
	masked := h[1]&0x80 != 0
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return f, err
		}
		h = append(h, ext...)
		n = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return f, err
		}
		h = append(h, ext...)
		n = binary.BigEndian.Uint64(ext)
	}
	if n > maxFrame {
		return f, errors.New("a WebSocket frame too large to relay")
	}
	var key []byte
	if masked {
		key = make([]byte, 4)
		if _, err := io.ReadFull(r, key); err != nil {
			return f, err
		}
		h = append(h, key...)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return f, err
	}
	f.raw = append(h, payload...)
	if masked {
		for i := range payload {
			payload[i] ^= key[i%4]
		}
	}
	f.payload = payload
	return f, nil
}

// writeFrame writes a server's frame, unmasked.
func writeFrame(w io.Writer, op byte, data []byte) error {
	h := []byte{0x80 | op}
	switch n := len(data); {
	case n < 126:
		h = append(h, byte(n))
	case n <= 0xffff:
		h = append(h, 126, byte(n>>8), byte(n))
	default:
		h = append(h, 127)
		h = binary.BigEndian.AppendUint64(h, uint64(n))
	}
	if _, err := w.Write(h); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// relayFrames copies frames from r to w, recording each message.
func relayFrames(r *bufio.Reader, w io.Writer, ex *Exchange, sent bool) {
	var partial []byte
	var partialOp byte
	for {
		f, err := readFrame(r)
		if err != nil {
			return
		}
		if _, err := w.Write(f.raw); err != nil {
			return
		}
		switch {
		case f.op == wsCont:
			partial = append(partial, f.payload...)
			if !f.fin {
				continue
			}
			ex.addMessage(proto.TrafficMessage{Sent: sent, Time: time.Now(), Kind: wsKinds[partialOp], Data: partial, Size: int64(len(partial))})
			partial = nil
		case !f.fin && (f.op == wsText || f.op == wsBinary):
			partial, partialOp = append([]byte(nil), f.payload...), f.op
		default:
			ex.addMessage(proto.TrafficMessage{Sent: sent, Time: time.Now(), Kind: wsKinds[f.op], Data: f.payload, Size: int64(len(f.payload))})
			if f.op == wsClose {
				return
			}
		}
	}
}
