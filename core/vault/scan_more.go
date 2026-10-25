package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/proto"
)

// scanSimple finds the tokens tools keep alone in a file of their own.
func scanSimple(home string, add func(proto.VaultFound)) {
	p := func(rel ...string) string { return filepath.Join(append([]string{home}, rel...)...) }
	token := func(platform, path, key string) {
		for _, line := range readLines(path) {
			if k, v, ok := keyValue(line); ok && k == key && v != "" {
				add(proto.VaultFound{Platform: platform, Fields: map[string]string{Lookup(platform).Fields[0].Key: v}, Source: path})
				return
			}
		}
	}
	jsonToken := func(platform, path string, find func(any) string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var v any
		if json.Unmarshal(data, &v) != nil {
			return
		}
		if t := find(v); t != "" {
			add(proto.VaultFound{Platform: platform, Fields: map[string]string{"token": t}, Source: path})
		}
	}
	if kv := iniSections(p(".pypirc"))["pypi"]; kv["password"] != "" {
		add(proto.VaultFound{Platform: "pypi", Fields: map[string]string{"token": kv["password"]}, Source: p(".pypirc")})
	}
	token("crates", p(".cargo", "credentials.toml"), "token")
	token("fly", p(".fly", "config.yml"), "access_token")
	token("digitalocean", p(".config", "doctl", "config.yaml"), "access-token")
	token("digitalocean", p("Library", "Application Support", "doctl", "config.yaml"), "access-token")
	token("hetzner", p(".config", "hcloud", "cli.toml"), "token")
	if data, err := os.ReadFile(p(".supabase", "access-token")); err == nil {
		add(proto.VaultFound{Platform: "supabase", Fields: map[string]string{"token": string(data)}, Source: p(".supabase", "access-token")})
	}
	for _, f := range []string{p(".local", "share", "com.vercel.cli", "auth.json"), p("Library", "Application Support", "com.vercel.cli", "auth.json")} {
		jsonToken("vercel", f, func(v any) string { return str(v, "token") })
	}
	jsonToken("railway", p(".railway", "config.json"), func(v any) string { return str(v, "user", "token") })
	for _, f := range []string{p(".netlify", "config.json"), p("Library", "Preferences", "netlify", "config.json"), p(".config", "netlify", "config.json")} {
		jsonToken("netlify", f, func(v any) string {
			users, _ := at(v, "users").(map[string]any)
			for _, u := range users {
				if t := str(u, "auth", "token"); t != "" {
					return t
				}
			}
			return ""
		})
	}
}

// at walks JSON objects by key.
func at(v any, keys ...string) any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func str(v any, keys ...string) string {
	s, _ := at(v, keys...).(string)
	return strings.TrimSpace(s)
}
