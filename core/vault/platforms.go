package vault

import (
	"slices"
	"strings"
)

// Platform is a service credentials log in to: the values a credential of
// it holds, the variables they set, where git and docker use them, how to
// check one still works and how its own command line logs in.
type Platform struct {
	ID    string
	Name  string
	Group string
	Type  string
	// Fields are the values a credential holds, the main one first.
	Fields []Field
	// Env is the variables a session gets: variable → field key, or a
	// value of its own after "=".
	Env map[string]string
	// GitHosts are the servers git logs in to with it, as GitUser (a
	// field key after "@": that field's value).
	GitHosts []string
	GitUser  string
	// Registries are the container registries docker logs in to with it.
	Registries []string
	// Check asks the platform whether a credential works, nil for none.
	Check *Check
	// Login is its command line's login, run in a session for the user.
	Login string
	// Link is where the user makes a token.
	Link string
	// Device logs in in the browser (OAuth's device flow), nil for none.
	Device *Device
}

// Field is a value of a credential.
type Field struct {
	Key, Label string
	Secret     bool
	Optional   bool
	Multiline  bool
	// File: a session gets the value in a file, its variable the path.
	File bool
}

// Check is a request whose success (2xx) means the credential works.
// "{key}" in URL, Header and Body is the field's value, "{key|default}"
// a default for an empty one.
type Check struct {
	Method string
	URL    string
	Header map[string]string
	// Basic is HTTP basic auth: user and password templates.
	Basic [2]string
	Body  string
	// Fail: a body containing it failed, though 2xx (GraphQL's errors).
	Fail string
}

// Platform groups, in the order the app shows them.
var Groups = []string{"Git & code", "Cloud", "Edge & hosting", "Databases", "Registries & packages", "AI APIs", "Other", "Generic"}

// Lookup returns platform id, or the generic API key for one unknown.
func Lookup(id string) Platform {
	if i := slices.IndexFunc(Platforms, func(p Platform) bool { return p.ID == id }); i >= 0 {
		return Platforms[i]
	}
	p := genericKey
	p.ID, p.Name = id, id
	return p
}

// Search returns the platforms whose name or ID has q, ignoring case.
func Search(q string) []Platform {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []Platform
	for _, p := range Platforms {
		if q == "" || strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(p.ID, q) {
			out = append(out, p)
		}
	}
	return out
}

// Common fields.
var (
	fToken    = Field{Key: "token", Label: "Token", Secret: true}
	fAPIKey   = Field{Key: "api_key", Label: "API key", Secret: true}
	fUsername = Field{Key: "username", Label: "Username"}
	fPassword = Field{Key: "password", Label: "Password", Secret: true}
	fURL      = Field{Key: "url", Label: "Server URL", Optional: true}
)

func bearer(url string) *Check {
	return &Check{URL: url, Header: map[string]string{"Authorization": "Bearer {token}"}}
}

var genericKey = Platform{ID: "api-key", Name: "API key", Group: "Generic", Type: TypeAPIKey,
	Fields: []Field{{Key: "value", Label: "Value", Secret: true}}, Env: map[string]string{"API_KEY": "value"}}
