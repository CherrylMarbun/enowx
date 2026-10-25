package intercept

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/enowx/enowx/proto"
)

// Send sends a request from the host as src, through the rules unless
// p.Direct, recorded like any exchange, and returns it once answered.
func (p *Proxy) Send(ctx context.Context, src Source, params proto.TrafficSendParams) (proto.TrafficSummary, error) {
	u, err := url.Parse(params.URL)
	if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
		return proto.TrafficSummary{}, fmt.Errorf("intercept: %q isn't an http or https URL", params.URL)
	}
	method := params.Method
	if method == "" {
		method = http.MethodGet
	}
	r, err := http.NewRequestWithContext(ctx, method, u.String(), io.NopCloser(bytes.NewReader(params.Body)))
	if err != nil {
		return proto.TrafficSummary{}, err
	}
	r.Header = headerMap(params.Headers)
	r.ContentLength = int64(len(params.Body))
	if len(params.Body) == 0 {
		r.Body = http.NoBody
	}
	r.Host = u.Host
	r.Proto, r.ProtoMajor, r.ProtoMinor = "HTTP/1.1", 1, 1
	src.NoRules = params.Direct
	w := &recorder{header: http.Header{}}
	ex := p.exchange(w, r, u, src, connInfo{src: src, scheme: u.Scheme, target: u.Host}, false)
	if ex == nil {
		return proto.TrafficSummary{}, fmt.Errorf("intercept: the request wasn't sent")
	}
	return ex.Summary(), nil
}

// recorder is the client's side of a request sent from the host: it
// keeps nothing, the exchange records what matters.
type recorder struct {
	header http.Header
	status int
}

func (w *recorder) Header() http.Header         { return w.header }
func (w *recorder) Write(p []byte) (int, error) { return len(p), nil }
func (w *recorder) WriteHeader(status int)      { w.status = status }
func (w *recorder) Flush()                      {}
