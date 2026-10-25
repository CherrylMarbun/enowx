package intercept

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Secrets are masked wherever an exchange is shown or sent without the
// user revealing them: in lists, to agents, in exports.

// secretName matches header, query and field names that carry secrets.
var secretName = regexp.MustCompile(`(?i)(^authorization$|^proxy-authorization$|^cookie$|^set-cookie$|token|secret|passw|api[-_]?key|apikey|session[-_]?id|^x-amz-security|credential|private[-_]?key|signature$)`)

// IsSecretName reports whether a header or field named name carries a
// secret.
func IsSecretName(name string) bool { return secretName.MatchString(name) }

// mask keeps a hint of a secret: its scheme ("Bearer") and its last four
// characters when it is long enough not to give much away.
func mask(v string) string {
	scheme, rest, ok := strings.Cut(v, " ")
	if !ok || len(scheme) > 12 {
		scheme, rest = "", v
	} else {
		scheme += " "
	}
	if len(rest) >= 16 {
		return scheme + "•••" + rest[len(rest)-4:]
	}
	return scheme + "•••"
}

// MaskHeaders masks the values of secret headers.
func MaskHeaders(hs []proto.Header) []proto.Header {
	out := make([]proto.Header, len(hs))
	for i, h := range hs {
		if IsSecretName(h.Name) {
			h.Value = mask(h.Value)
		}
		out[i] = h
	}
	return out
}

// MaskURL masks the values of secret query parameters and a URL's
// password.
func MaskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	changed := false
	if u.User != nil {
		if _, ok := u.User.Password(); ok {
			u.User = url.UserPassword(u.User.Username(), "•••")
			changed = true
		}
	}
	q := u.Query()
	for k, vs := range q {
		if IsSecretName(k) {
			for i := range vs {
				vs[i] = mask(vs[i])
			}
			changed = true
		}
	}
	if !changed {
		return raw
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// MaskBody masks secret fields of a JSON or URL-encoded body; others are
// returned as they are.
func MaskBody(kind string, data []byte) []byte {
	switch kind {
	case "json":
		var v any
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if dec.Decode(&v) != nil {
			return data
		}
		if !maskJSON(v) {
			return data
		}
		out, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return data
		}
		return out
	case "form":
		q, err := url.ParseQuery(string(data))
		if err != nil {
			return data
		}
		changed := false
		for k, vs := range q {
			if IsSecretName(k) {
				for i := range vs {
					vs[i] = mask(vs[i])
				}
				changed = true
			}
		}
		if changed {
			return []byte(q.Encode())
		}
	}
	return data
}

func maskJSON(v any) bool {
	changed := false
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if s, ok := x.(string); ok && IsSecretName(k) {
				t[k] = mask(s)
				changed = true
				continue
			}
			changed = maskJSON(x) || changed
		}
	case []any:
		for _, x := range t {
			changed = maskJSON(x) || changed
		}
	}
	return changed
}
