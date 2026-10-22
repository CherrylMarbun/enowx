package envsync

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/enowx/enowx/proto"
)

// secretFile says why a file looks like it holds a secret, or "".
func secretFile(f proto.EnvFile) string {
	base := strings.ToLower(path.Base(f.Path))
	switch {
	case base == ".env" || strings.HasPrefix(base, ".env."),
		strings.HasSuffix(base, ".pem"), strings.HasSuffix(base, ".key"), strings.HasSuffix(base, ".p12"),
		strings.HasPrefix(base, "id_rsa"), strings.HasPrefix(base, "id_ed25519"), strings.HasPrefix(base, "id_ecdsa"),
		strings.Contains(base, "credential"), strings.Contains(base, "secret"):
		return "file " + path.Base(f.Path)
	}
	return ""
}

// settingsSecrets names the environment variables a settings file sets.
func settingsSecrets(files []proto.EnvFile) []string {
	if len(files) != 1 {
		return nil
	}
	var v struct {
		Env map[string]any `json:"env"`
	}
	if json.Unmarshal(files[0].Data, &v) != nil {
		return nil
	}
	var out []string
	for _, k := range sortedKeys(v.Env) {
		out = append(out, "env "+k)
	}
	return out
}

var secretWord = regexp.MustCompile(`(?i)(token|secret|key|password|passwd|auth|bearer|credential)`)

// mcpSecrets names what in an MCP server's config looks secret: its
// environment variables and headers, and URL parameters or arguments that
// carry keys.
func mcpSecrets(server map[string]any) []string {
	var out []string
	for _, field := range []string{"env", "environment", "headers", "http_headers", "env_http_headers"} {
		if m, ok := server[field].(map[string]any); ok {
			for _, k := range sortedKeys(m) {
				out = append(out, field+" "+k)
			}
		}
	}
	if u, ok := server["url"].(string); ok {
		if parsed, err := url.Parse(u); err == nil {
			for _, k := range sortedKeys(parsed.Query()) {
				if secretWord.MatchString(k) {
					out = append(out, "url parameter "+k)
				}
			}
		}
	}
	if args, ok := server["args"].([]any); ok {
		for i, a := range args {
			s, _ := a.(string)
			name, value, isFlag := strings.Cut(strings.TrimLeft(s, "-"), "=")
			flagged := strings.HasPrefix(s, "-") && secretWord.MatchString(name)
			if flagged && isFlag && value != "" {
				out = append(out, "argument "+s[:len(s)-len(value)-1])
			} else if flagged && !isFlag && i+1 < len(args) {
				out = append(out, "argument "+s)
			}
		}
	}
	return out
}
