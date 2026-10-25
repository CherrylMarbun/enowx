package intercept

import (
	"mime"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// Limits of what a store keeps: the last exchanges, the bodies' bytes in
// all, and each body (past which only its size is counted).
const (
	DefaultMaxExchanges = 2000
	DefaultMaxBytes     = 256 << 20
	DefaultMaxBody      = 16 << 20
	maxMessages         = 5000
)

// Exchange is one request and its response as the proxy saw them. Its
// fields belong to its mutex; the proxy writes them as the exchange goes,
// readers take a copy.
type Exchange struct {
	mu       sync.Mutex
	sum      proto.TrafficSummary
	reqHead  http.Header
	resHead  http.Header
	trailers http.Header
	req, res body
	timings  proto.TrafficTimings
	tls      *proto.TrafficTLS
	remote   string
	messages []proto.TrafficMessage
	store    *Store
}

// body is a body as kept: its bytes up to the store's limit, as received
// (still content-encoded).
type body struct {
	data      []byte
	size      int64
	truncated bool
	ctype     string
	encoding  string
}

// Store keeps the exchanges of a host's proxy, oldest first. It is safe for
// use by several goroutines; OnChange is called without its lock held.
type Store struct {
	MaxExchanges int
	MaxBytes     int64
	MaxBody      int64
	// OnChange learns of each exchange started, changed or done.
	OnChange func(proto.TrafficSummary)

	mu    sync.Mutex
	next  int64
	list  []*Exchange
	byID  map[int64]*Exchange
	bytes int64
}

// NewStore is a store with the default limits.
func NewStore() *Store {
	return &Store{MaxExchanges: DefaultMaxExchanges, MaxBytes: DefaultMaxBytes, MaxBody: DefaultMaxBody, byID: map[int64]*Exchange{}}
}

// begin records a new exchange.
func (s *Store) begin(sum proto.TrafficSummary) *Exchange {
	s.mu.Lock()
	s.next++
	sum.ID = s.next
	if sum.Started.IsZero() {
		sum.Started = time.Now()
	}
	if sum.State == "" {
		sum.State = proto.TrafficPending
	}
	ex := &Exchange{sum: sum, store: s}
	s.list = append(s.list, ex)
	s.byID[sum.ID] = ex
	s.evictLocked()
	s.mu.Unlock()
	ex.changed()
	return ex
}

// evictLocked drops the oldest unpinned exchanges past the limits.
func (s *Store) evictLocked() {
	for i := 0; i < len(s.list) && (len(s.list) > s.MaxExchanges || s.bytes > s.MaxBytes); {
		ex := s.list[i]
		ex.mu.Lock()
		pinned, done := ex.sum.Pinned, ex.sum.State == proto.TrafficDone || ex.sum.State == proto.TrafficFailed
		size := int64(len(ex.req.data) + len(ex.res.data))
		ex.mu.Unlock()
		if pinned || !done {
			i++
			continue
		}
		s.list = slices.Delete(s.list, i, i+1)
		delete(s.byID, ex.sum.ID)
		s.bytes -= size
	}
}

// grow counts n more bytes kept, evicting what is past the limits.
func (s *Store) grow(n int64) {
	s.mu.Lock()
	s.bytes += n
	if s.bytes > s.MaxBytes {
		s.evictLocked()
	}
	s.mu.Unlock()
}

// Get returns exchange id.
func (s *Store) Get(id int64) (*Exchange, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ex, ok := s.byID[id]
	return ex, ok
}

// All lists the exchanges kept, oldest first.
func (s *Store) All() []*Exchange {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.list)
}

// Stats are how many exchanges and body bytes the store keeps.
func (s *Store) Stats() (int, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.list), s.bytes
}

// Clear drops the exchanges match accepts, pinned ones only when pinned.
func (s *Store) Clear(match func(*Exchange) bool, pinned bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.list[:0]
	for _, ex := range s.list {
		ex.mu.Lock()
		keep := (ex.sum.Pinned && !pinned) || ex.sum.State == proto.TrafficPending || ex.sum.State == proto.TrafficHeld
		size := int64(len(ex.req.data) + len(ex.res.data))
		id := ex.sum.ID
		ex.mu.Unlock()
		if keep || (match != nil && !match(ex)) {
			kept = append(kept, ex)
			continue
		}
		delete(s.byID, id)
		s.bytes -= size
	}
	for i := len(kept); i < len(s.list); i++ {
		s.list[i] = nil
	}
	s.list = kept
}

// Summary is the exchange as a list shows it.
func (ex *Exchange) Summary() proto.TrafficSummary {
	ex.mu.Lock()
	defer ex.mu.Unlock()
	return ex.sum
}

// ID is the exchange's id.
func (ex *Exchange) ID() int64 { return ex.Summary().ID }

// update changes the exchange under its lock, then tells the store.
func (ex *Exchange) update(fn func(ex *Exchange)) {
	ex.mu.Lock()
	fn(ex)
	ex.mu.Unlock()
	ex.changed()
}

func (ex *Exchange) changed() {
	if ex.store != nil && ex.store.OnChange != nil {
		ex.store.OnChange(ex.Summary())
	}
}

// keep appends p to a body, up to the store's limit per body.
func (ex *Exchange) keep(b *body, p []byte) {
	ex.mu.Lock()
	b.size += int64(len(p))
	room := ex.store.MaxBody - int64(len(b.data))
	kept := 0
	if room > 0 {
		kept = int(min(room, int64(len(p))))
		b.data = append(b.data, p[:kept]...)
	}
	if kept < len(p) {
		b.truncated = true
	}
	ex.mu.Unlock()
	if kept > 0 {
		ex.store.grow(int64(kept))
	}
}

// addMessage records a message of a stream.
func (ex *Exchange) addMessage(m proto.TrafficMessage) {
	ex.mu.Lock()
	if len(ex.messages) < maxMessages {
		if int64(len(m.Data)) > ex.store.MaxBody {
			m.Data = m.Data[:ex.store.MaxBody]
		}
		ex.messages = append(ex.messages, m)
	}
	ex.sum.Messages++
	ex.mu.Unlock()
	ex.changed()
}

// Details is the exchange in full but its bodies, secrets masked unless
// reveal.
func (ex *Exchange) Details(reveal bool) proto.TrafficExchange {
	ex.mu.Lock()
	defer ex.mu.Unlock()
	d := proto.TrafficExchange{
		TrafficSummary: ex.sum,
		ReqHeaders:     headerList(ex.reqHead),
		ResHeaders:     headerList(ex.resHead),
		Trailers:       headerList(ex.trailers),
		ReqType:        ex.req.ctype,
		ResType:        ex.res.ctype,
		ReqKept:        !ex.req.truncated,
		ResKept:        !ex.res.truncated,
		Timings:        ex.timings,
		TLS:            ex.tls,
		Remote:         ex.remote,
	}
	if !reveal {
		d.ReqHeaders, d.ResHeaders = MaskHeaders(d.ReqHeaders), MaskHeaders(d.ResHeaders)
		d.URL = MaskURL(d.URL)
		d.Masked = true
	}
	return d
}

// Messages lists the stream's messages, limit from offset.
func (ex *Exchange) Messages(offset, limit int) []proto.TrafficMessage {
	ex.mu.Lock()
	defer ex.mu.Unlock()
	if offset >= len(ex.messages) {
		return nil
	}
	end := len(ex.messages)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return slices.Clone(ex.messages[offset:end])
}

// rawBody is a copy of a body as kept, with its encoding and type.
func (ex *Exchange) rawBody(which string) body {
	ex.mu.Lock()
	defer ex.mu.Unlock()
	b := ex.req
	if which == "response" {
		b = ex.res
	}
	b.data = slices.Clone(b.data)
	return b
}

// headerList lists h's fields, by name, each value apart.
func headerList(h http.Header) []proto.Header {
	if len(h) == 0 {
		return nil
	}
	names := make([]string, 0, len(h))
	for n := range h {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []proto.Header
	for _, n := range names {
		for _, v := range h[n] {
			out = append(out, proto.Header{Name: n, Value: v})
		}
	}
	return out
}

// kindOf is what a body is, for the list and its viewer: json, html, xml,
// image, text, form, multipart, grpc, sse, js, css, font, binary.
func kindOf(contentType string) string {
	mt, _, _ := mime.ParseMediaType(contentType)
	switch {
	case mt == "":
		return ""
	case strings.HasPrefix(mt, "application/grpc"):
		return "grpc"
	case mt == "text/event-stream":
		return "sse"
	case mt == "application/json" || strings.HasSuffix(mt, "+json") || mt == "application/graphql-response+json":
		return "json"
	case mt == "text/html":
		return "html"
	case mt == "application/xml" || mt == "text/xml" || strings.HasSuffix(mt, "+xml"):
		return "xml"
	case strings.HasPrefix(mt, "image/"):
		return "image"
	case mt == "application/x-www-form-urlencoded":
		return "form"
	case strings.HasPrefix(mt, "multipart/"):
		return "multipart"
	case mt == "text/javascript" || mt == "application/javascript":
		return "js"
	case mt == "text/css":
		return "css"
	case strings.HasPrefix(mt, "font/"):
		return "font"
	case strings.HasPrefix(mt, "text/"):
		return "text"
	case mt == "application/graphql":
		return "graphql"
	}
	return "binary"
}
