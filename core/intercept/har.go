package intercept

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enowx/enowx/proto"
)

// HAR 1.2 (the HTTP Archive format browsers export), with WebSocket
// messages as browsers write them (_webSocketMessages).

type harFile struct {
	Log harLog `json:"log"`
}

type harLog struct {
	Version string     `json:"version"`
	Creator harCreator `json:"creator"`
	Entries []harEntry `json:"entries"`
}

type harCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type harEntry struct {
	Started  time.Time      `json:"startedDateTime"`
	Time     float64        `json:"time"`
	Request  harRequest     `json:"request"`
	Response harResponse    `json:"response"`
	Cache    struct{}       `json:"cache"`
	Timings  harTimings     `json:"timings"`
	Server   string         `json:"serverIPAddress,omitempty"`
	Messages []harWSMessage `json:"_webSocketMessages,omitempty"`
}

type harNV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harRequest struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	HTTPVersion string       `json:"httpVersion"`
	Headers     []harNV      `json:"headers"`
	Query       []harNV      `json:"queryString"`
	Cookies     []harNV      `json:"cookies"`
	HeadersSize int          `json:"headersSize"`
	BodySize    int64        `json:"bodySize"`
	PostData    *harPostData `json:"postData,omitempty"`
}

type harPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Encoding string `json:"encoding,omitempty"`
}

type harResponse struct {
	Status      int        `json:"status"`
	StatusText  string     `json:"statusText"`
	HTTPVersion string     `json:"httpVersion"`
	Headers     []harNV    `json:"headers"`
	Cookies     []harNV    `json:"cookies"`
	Content     harContent `json:"content"`
	RedirectURL string     `json:"redirectURL"`
	HeadersSize int        `json:"headersSize"`
	BodySize    int64      `json:"bodySize"`
}

type harContent struct {
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

type harTimings struct {
	Blocked float64 `json:"blocked"`
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	SSL     float64 `json:"ssl"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
}

type harWSMessage struct {
	Type   string  `json:"type"` // send, receive
	Time   float64 `json:"time"` // seconds since the epoch
	Opcode int     `json:"opcode"`
	Data   string  `json:"data"`
}

// ExportHAR writes exchanges as HAR, secrets masked unless reveal.
func ExportHAR(exs []*Exchange, version string, reveal bool) ([]byte, error) {
	f := harFile{Log: harLog{Version: "1.2", Creator: harCreator{Name: "enowX", Version: version}, Entries: []harEntry{}}}
	for _, ex := range exs {
		d := ex.Details(reveal)
		if d.Kind == proto.TrafficTunnel {
			continue
		}
		req, res := ex.rawBody("request"), ex.rawBody("response")
		reqData, _ := decode(req.data, req.encoding)
		resData, _ := decode(res.data, res.encoding)
		if !reveal {
			reqData, resData = MaskBody(kindOf(req.ctype), reqData), MaskBody(kindOf(res.ctype), resData)
		}
		e := harEntry{
			Started: d.Started, Time: float64(d.Duration),
			Request: harRequest{
				Method: d.Method, URL: d.URL, HTTPVersion: firstProto(d.Proto), Headers: harHeaders(d.ReqHeaders),
				Query: harQuery(d.URL), Cookies: []harNV{}, HeadersSize: -1, BodySize: req.size,
			},
			Response: harResponse{
				Status: d.Status, StatusText: http.StatusText(d.Status), HTTPVersion: lastProto(d.Proto),
				Headers: harHeaders(d.ResHeaders), Cookies: []harNV{}, HeadersSize: -1, BodySize: res.size,
				Content: harContent{Size: int64(len(resData)), MimeType: res.ctype},
			},
			Timings: harTimingsOf(d.Timings),
			Server:  hostOnly(d.Remote),
		}
		if len(reqData) > 0 || req.ctype != "" {
			text, enc := harText(reqData)
			e.Request.PostData = &harPostData{MimeType: req.ctype, Text: text, Encoding: enc}
		}
		e.Response.Content.Text, e.Response.Content.Encoding = harText(resData)
		if d.Kind == proto.TrafficWS {
			for _, m := range ex.Messages(0, 0) {
				typ, op := "receive", 1
				if m.Sent {
					typ = "send"
				}
				if m.Kind == "binary" {
					op = 2
				}
				e.Messages = append(e.Messages, harWSMessage{Type: typ, Time: float64(m.Time.UnixNano()) / 1e9, Opcode: op, Data: string(m.Data)})
			}
		}
		f.Log.Entries = append(f.Log.Entries, e)
	}
	return json.MarshalIndent(f, "", "  ")
}

// ImportHAR adds a HAR's entries to the store as exchanges of source.
func (s *Store) ImportHAR(data []byte, source string) ([]proto.TrafficSummary, error) {
	var f harFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("intercept: not a HAR file: %w", err)
	}
	var out []proto.TrafficSummary
	for _, e := range f.Log.Entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil {
			continue
		}
		reqHead, resHead := http.Header{}, http.Header{}
		for _, h := range e.Request.Headers {
			if !strings.HasPrefix(h.Name, ":") {
				reqHead.Add(h.Name, h.Value)
			}
		}
		for _, h := range e.Response.Headers {
			if !strings.HasPrefix(h.Name, ":") {
				resHead.Add(h.Name, h.Value)
			}
		}
		kind := proto.TrafficHTTP
		if len(e.Messages) > 0 {
			kind = proto.TrafficWS
		}
		ex := s.begin(proto.TrafficSummary{Source: source, Started: e.Started, Method: e.Request.Method, URL: e.Request.URL, Host: u.Hostname(), Path: u.RequestURI(), Proto: e.Request.HTTPVersion, Kind: kind})
		var reqBody, resBody []byte
		if pd := e.Request.PostData; pd != nil {
			reqBody = harBytes(pd.Text, pd.Encoding)
		}
		resBody = harBytes(e.Response.Content.Text, e.Response.Content.Encoding)
		ex.keep(&ex.req, reqBody)
		ex.keep(&ex.res, resBody)
		for _, m := range e.Messages {
			k := "text"
			if m.Opcode == 2 {
				k = "binary"
			}
			ex.addMessage(proto.TrafficMessage{Sent: m.Type == "send", Time: time.Unix(0, int64(m.Time*1e9)), Kind: k, Data: []byte(m.Data), Size: int64(len(m.Data))})
		}
		ex.update(func(ex *Exchange) {
			ex.reqHead, ex.resHead = reqHead, resHead
			// Bodies in a HAR are decoded already.
			ex.reqHead.Del("Content-Encoding")
			ex.resHead.Del("Content-Encoding")
			ex.req.ctype, ex.res.ctype = reqHead.Get("Content-Type"), e.Response.Content.MimeType
			ex.sum.Status, ex.sum.Type = e.Response.Status, kindOf(e.Response.Content.MimeType)
			ex.sum.State, ex.sum.Duration = proto.TrafficDone, int64(e.Time)
			ex.sum.ReqSize, ex.sum.ResSize = ex.req.size, ex.res.size
		})
		out = append(out, ex.Summary())
	}
	return out, nil
}

func harHeaders(hs []proto.Header) []harNV {
	out := make([]harNV, 0, len(hs))
	for _, h := range hs {
		out = append(out, harNV{Name: h.Name, Value: h.Value})
	}
	return out
}

func harQuery(raw string) []harNV {
	out := []harNV{}
	u, err := url.Parse(raw)
	if err != nil {
		return out
	}
	for k, vs := range u.Query() {
		for _, v := range vs {
			out = append(out, harNV{Name: k, Value: v})
		}
	}
	return out
}

// harText is a body as HAR text: as it is when it is text, else base64.
func harText(b []byte) (string, string) {
	if utf8.Valid(b) {
		return string(b), ""
	}
	return base64.StdEncoding.EncodeToString(b), "base64"
}

func harBytes(text, encoding string) []byte {
	if encoding == "base64" {
		b, _ := base64.StdEncoding.DecodeString(text)
		return b
	}
	return []byte(text)
}

func harTimingsOf(t proto.TrafficTimings) harTimings {
	h := harTimings{Blocked: -1, DNS: -1, Connect: -1, SSL: -1}
	if t.Connect > 0 {
		h.Connect = float64(t.Connect)
	}
	if t.TLS > 0 && t.TLS >= t.Connect {
		h.SSL = float64(t.TLS - t.Connect)
	}
	if t.Sent > 0 {
		h.Send = float64(max(0, t.Sent-max(t.TLS, t.Connect)))
	}
	if t.FirstByte > 0 {
		h.Wait = float64(max(0, t.FirstByte-t.Sent))
	}
	if t.Done > 0 && t.FirstByte > 0 {
		h.Receive = float64(max(0, t.Done-t.FirstByte))
	}
	return h
}

func firstProto(p string) string {
	a, _, _ := strings.Cut(p, " → ")
	return a
}

func lastProto(p string) string {
	if _, b, ok := strings.Cut(p, " → "); ok {
		return b
	}
	return p
}

func hostOnly(addr string) string {
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return strings.Trim(addr[:i], "[]")
	}
	return addr
}
