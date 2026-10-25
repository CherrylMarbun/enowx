package intercept

import (
	"fmt"
	"sort"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Snippet languages, for Copy as.
var SnippetLanguages = []string{"curl", "fetch", "python", "go", "httpie", "powershell"}

// skipInSnippet are headers a client sets itself.
var skipInSnippet = map[string]bool{"Content-Length": true, "Host": true, "Accept-Encoding": true, "Connection": true}

// Snippet writes a request as code in lang.
func Snippet(lang, method, url string, headers []proto.Header, body []byte) (string, error) {
	var hs []proto.Header
	for _, h := range headers {
		if !skipInSnippet[h.Name] && !strings.HasPrefix(h.Name, ":") {
			hs = append(hs, h)
		}
	}
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].Name < hs[j].Name })
	b := string(body)
	switch lang {
	case "curl":
		var s strings.Builder
		fmt.Fprintf(&s, "curl -X %s %s", method, shq(url))
		for _, h := range hs {
			fmt.Fprintf(&s, " \\\n  -H %s", shq(h.Name+": "+h.Value))
		}
		if b != "" {
			fmt.Fprintf(&s, " \\\n  --data-raw %s", shq(b))
		}
		return s.String(), nil
	case "httpie":
		var s strings.Builder
		fmt.Fprintf(&s, "http %s %s", method, shq(url))
		for _, h := range hs {
			fmt.Fprintf(&s, " \\\n  %s", shq(h.Name+":"+h.Value))
		}
		if b != "" {
			return "printf %s " + shq(b) + " | " + s.String(), nil
		}
		return s.String(), nil
	case "fetch":
		var s strings.Builder
		fmt.Fprintf(&s, "await fetch(%s, {\n  method: %s,\n", jsq(url), jsq(method))
		if len(hs) > 0 {
			s.WriteString("  headers: {\n")
			for _, h := range hs {
				fmt.Fprintf(&s, "    %s: %s,\n", jsq(h.Name), jsq(h.Value))
			}
			s.WriteString("  },\n")
		}
		if b != "" {
			fmt.Fprintf(&s, "  body: %s,\n", jsq(b))
		}
		s.WriteString("});")
		return s.String(), nil
	case "python":
		var s strings.Builder
		s.WriteString("import requests\n\n")
		fmt.Fprintf(&s, "response = requests.request(\n    %s,\n    %s,\n", pyq(method), pyq(url))
		if len(hs) > 0 {
			s.WriteString("    headers={\n")
			for _, h := range hs {
				fmt.Fprintf(&s, "        %s: %s,\n", pyq(h.Name), pyq(h.Value))
			}
			s.WriteString("    },\n")
		}
		if b != "" {
			fmt.Fprintf(&s, "    data=%s,\n", pyq(b))
		}
		s.WriteString(")\nprint(response.status_code, response.text)")
		return s.String(), nil
	case "go":
		var s strings.Builder
		body := "nil"
		if b != "" {
			body = "strings.NewReader(" + fmt.Sprintf("%q", b) + ")"
		}
		fmt.Fprintf(&s, "req, err := http.NewRequest(%q, %q, %s)\nif err != nil {\n\treturn err\n}\n", method, url, body)
		for _, h := range hs {
			fmt.Fprintf(&s, "req.Header.Set(%q, %q)\n", h.Name, h.Value)
		}
		s.WriteString("resp, err := http.DefaultClient.Do(req)\nif err != nil {\n\treturn err\n}\ndefer resp.Body.Close()")
		return s.String(), nil
	case "powershell":
		var s strings.Builder
		if len(hs) > 0 {
			s.WriteString("$headers = @{\n")
			for _, h := range hs {
				fmt.Fprintf(&s, "  %s = %s\n", psq(h.Name), psq(h.Value))
			}
			s.WriteString("}\n")
		}
		fmt.Fprintf(&s, "Invoke-WebRequest -Uri %s -Method %s", psq(url), method)
		if len(hs) > 0 {
			s.WriteString(" -Headers $headers")
		}
		if b != "" {
			fmt.Fprintf(&s, " -Body %s", psq(b))
		}
		return s.String(), nil
	}
	return "", fmt.Errorf("no snippets in %q: %s", lang, strings.Join(SnippetLanguages, ", "))
}

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
func psq(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func pyq(s string) string { return fmt.Sprintf("%q", s) }

func jsq(s string) string {
	r := strings.NewReplacer(`\`, `\\`, "'", `\'`, "\n", `\n`, "\r", `\r`)
	return "'" + r.Replace(s) + "'"
}
