package intercept

import (
	"net/http"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// The filter language picks exchanges by domain and subdomain, status,
// type, header, body and size, negated with "-".
func TestFilter(t *testing.T) {
	s := NewStore()
	add := func(url, host, path string, status int, ctype, body string) {
		ex := s.begin(proto.TrafficSummary{Method: "GET", URL: url, Host: host, Path: path, Kind: proto.TrafficHTTP})
		ex.keep(&ex.res, []byte(body))
		ex.update(func(ex *Exchange) {
			ex.resHead = http.Header{"Content-Type": {ctype}}
			ex.res.ctype = ctype
			ex.sum.Status, ex.sum.Type, ex.sum.State = status, kindOf(ctype), proto.TrafficDone
			ex.sum.ResSize, ex.sum.Duration = int64(len(body)), int64(len(body))
		})
	}
	add("https://example.com/", "example.com", "/", 200, "text/html", "<h1>home</h1>")
	add("https://api.example.com/v1/users?page=2", "api.example.com", "/v1/users?page=2", 200, "application/json", `{"users":[]}`)
	add("https://cdn.example.com/a.png", "cdn.example.com", "/a.png", 404, "image/png", "")
	add("https://other.org/x", "other.org", "/x", 500, "application/json", `{"error":"invalid token"}`)
	cases := map[string][]string{
		"domain:example.com":           {"example.com", "api.example.com", "cdn.example.com"},
		"host:*.example.com":           {"api.example.com", "cdn.example.com"},
		"host:example.com":             {"example.com"},
		"-domain:example.com":          {"other.org"},
		"status:4xx":                   {"cdn.example.com"},
		"status:>=500":                 {"other.org"},
		"type:json domain:example.com": {"api.example.com"},
		"header:content-type=image":    {"cdn.example.com"},
		`body:"invalid token"`:         {"other.org"},
		"path:/v1/* query:page=2":      {"api.example.com"},
		"size:>10 -type:html":          {"api.example.com", "other.org"},
		"users":                        {"api.example.com"},
		"":                             {"example.com", "api.example.com", "cdn.example.com", "other.org"},
	}
	for q, want := range cases {
		f, err := ParseFilter(q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		var got []string
		for _, ex := range s.All() {
			if f.Match(ex) {
				got = append(got, ex.Summary().Host)
			}
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: got %v, want %v", q, got, want)
		}
	}
	if _, err := ParseFilter("status:abc"); err == nil {
		t.Error("a bad status parsed")
	}
}

// HAR export reads back by import, bodies and secrets as they should be.
func TestHARRoundTrip(t *testing.T) {
	s := NewStore()
	ex := s.begin(proto.TrafficSummary{Method: "POST", URL: "https://api.example.com/login", Host: "api.example.com", Path: "/login", Kind: proto.TrafficHTTP, Proto: "HTTP/2"})
	ex.keep(&ex.req, []byte(`{"user":"a","password":"hunter2hunter2hunter2"}`))
	ex.keep(&ex.res, []byte(`{"ok":true}`))
	ex.update(func(ex *Exchange) {
		ex.reqHead = http.Header{"Content-Type": {"application/json"}, "Authorization": {"Bearer secretsecretsecretsecret"}}
		ex.resHead = http.Header{"Content-Type": {"application/json"}}
		ex.req.ctype, ex.res.ctype = "application/json", "application/json"
		ex.sum.Status, ex.sum.State = 200, proto.TrafficDone
	})
	data, err := ExportHAR([]*Exchange{ex}, "test", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "hunter2") || strings.Contains(string(data), "secretsecretsecret") {
		t.Fatalf("secrets in the export: %s", data)
	}
	other := NewStore()
	got, err := other.ImportHAR(data, "import")
	if err != nil || len(got) != 1 || got[0].Status != 200 || got[0].Method != "POST" {
		t.Fatalf("%+v %v", got, err)
	}
	imported, _ := other.Get(got[0].ID)
	if b := imported.rawBody("response"); string(b.data) != `{"ok":true}` {
		t.Fatalf("body %q", b.data)
	}
}

// A request reads back as curl with its headers and body quoted.
func TestSnippet(t *testing.T) {
	got, err := Snippet("curl", "POST", "https://api.example.com/x", []proto.Header{{Name: "Content-Type", Value: "application/json"}, {Name: "Content-Length", Value: "9"}}, []byte(`{"a":"it's"}`))
	want := "curl -X POST 'https://api.example.com/x' \\\n  -H 'Content-Type: application/json' \\\n  --data-raw '{\"a\":\"it'\\''s\"}'"
	if err != nil || got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	for _, lang := range SnippetLanguages {
		if _, err := Snippet(lang, "GET", "https://x", nil, nil); err != nil {
			t.Error(err)
		}
	}
}
