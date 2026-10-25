package vault

import (
	"bufio"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// EnvNames is the variables the credential sets in a session, sorted.
func (c *Credential) EnvNames() []string {
	env, files := c.Lend()
	names := make([]string, 0, len(env)+len(files))
	for k := range env {
		names = append(names, k)
	}
	for k := range files {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

// Lend is what a session gets of the credential: variables, and files
// whose variable holds their path (the daemon writes them).
func (c *Credential) Lend() (env, files map[string]string) {
	env, files = map[string]string{}, map[string]string{}
	if c.Type == TypeEnvFile {
		for k, v := range ParseEnv(c.Fields["env"]) {
			env[k] = v
		}
		return env, files
	}
	p := Lookup(c.Platform)
	mapping := p.Env
	if c.Env != nil {
		mapping = c.Env
	}
	for name, src := range mapping {
		if v, ok := strings.CutPrefix(src, "="); ok {
			env[name] = v
			continue
		}
		v := c.Fields[src]
		if v == "" {
			continue
		}
		if fieldOf(p, src).File {
			files[name] = v
		} else {
			env[name] = v
		}
	}
	return env, files
}

func fieldOf(p Platform, key string) Field {
	for _, f := range p.Fields {
		if f.Key == key {
			return f
		}
	}
	return Field{Key: key}
}

// ParseEnv reads NAME=value lines, as a .env file: comments, blank lines
// and "export " skipped, quotes around a value taken off.
func ParseEnv(text string) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		out[name] = value
	}
	return out
}

// Secrets is every value of the credential worth hiding, to mask in what
// a command prints.
func (c *Credential) Secrets() []string {
	var out []string
	p := Lookup(c.Platform)
	for k, v := range c.Fields {
		if v == "" {
			continue
		}
		if c.Type == TypeEnvFile {
			for _, ev := range ParseEnv(v) {
				out = append(out, ev)
			}
			continue
		}
		if f := fieldOf(p, k); f.Secret || len(p.Fields) == 0 {
			out = append(out, v)
		}
	}
	return out
}

// gitHosts is where git logs in with it: the platform's servers, and the
// one of its URL field.
func (c *Credential) gitHosts(p Platform) []string {
	hosts := slices.Clone(p.GitHosts)
	if p.GitUser != "" {
		if u, err := url.Parse(c.Fields["url"]); err == nil && u.Host != "" {
			hosts = append(hosts, u.Host)
		}
	}
	return hosts
}

func (c *Credential) registries(p Platform) []string { return slices.Clone(p.Registries) }

// GitLogin is the username and password git gets for server host, if the
// credential logs in there.
func (c *Credential) GitLogin(host string) (user, secret string, ok bool) {
	p := Lookup(c.Platform)
	if !slices.ContainsFunc(c.gitHosts(p), func(h string) bool { return strings.EqualFold(h, host) }) {
		return "", "", false
	}
	return c.login(p.GitUser)
}

// DockerLogin is the login docker gets for registry, if the credential
// logs in there.
func (c *Credential) DockerLogin(registry string) (user, secret string, ok bool) {
	p := Lookup(c.Platform)
	want := registryHost(registry)
	if !slices.ContainsFunc(p.Registries, func(r string) bool { return registryHost(r) == want }) {
		return "", "", false
	}
	return c.login("@username")
}

// registryHost is a registry's server: "https://index.docker.io/v1/" is
// "index.docker.io".
func registryHost(r string) string {
	r = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(r), "https://"), "http://")
	r, _, _ = strings.Cut(r, "/")
	return r
}

func (c *Credential) login(user string) (string, string, bool) {
	if f, ok := strings.CutPrefix(user, "@"); ok {
		user = c.Fields[f]
	}
	if user == "" {
		user = c.Account
	}
	if user == "" {
		user = "x-access-token"
	}
	secret := c.main()
	if secret == "" {
		return "", "", false
	}
	return user, secret, true
}

// Decision says whether a host may have a credential.
type Decision int

// Decisions.
const (
	Deny  Decision = iota
	Ask            // the user decides, the first time on a host
	Allow          // allowed on the host already
)

// Allowed decides whether session of project on host hostID may have the
// credential, and why not.
func (c *Credential) Allowed(hostID, project string, now time.Time) (Decision, string) {
	switch c.State(now) {
	case "revoked":
		return Deny, fmt.Sprintf("%s was revoked", c.Name)
	case "expired":
		return Deny, fmt.Sprintf("%s expired on %s: rotate it in the app", c.Name, c.Expires.Format("2 Jan 2006"))
	}
	if len(c.Projects) > 0 && !slices.ContainsFunc(c.Projects, func(p string) bool { return sameProject(p, project) }) {
		return Deny, fmt.Sprintf("%s is only for %s", c.Name, strings.Join(c.Projects, ", "))
	}
	if slices.Contains(c.Hosts, hostID) {
		return Allow, ""
	}
	if c.HostsOnly {
		return Deny, fmt.Sprintf("%s may not be used on this host", c.Name)
	}
	return Ask, ""
}

// sameProject: a project scope is a folder's path, or its name.
func sameProject(scope, project string) bool {
	if project == "" {
		return false
	}
	scope, project = strings.TrimRight(scope, "/"), strings.TrimRight(project, "/")
	return scope == project || (!strings.ContainsAny(scope, `/\`) && filepath.Base(project) == scope)
}

// Pick returns the credentials for a request: the one named, else those
// of platform (or every one, platform empty) that match, the ones scoped
// to project first, then the default.
func (v *Vault) Pick(name, platform, project string, match func(*Credential) bool) []*Credential {
	if name != "" {
		if c, ok := v.Find(name); ok {
			return []*Credential{c}
		}
		return nil
	}
	var out []*Credential
	for i := range v.Credentials {
		c := &v.Credentials[i]
		if (platform == "" || c.Platform == platform) && (match == nil || match(c)) {
			out = append(out, c)
		}
	}
	rank := func(c *Credential) int {
		switch {
		case len(c.Projects) > 0 && slices.ContainsFunc(c.Projects, func(p string) bool { return sameProject(p, project) }):
			return 0
		case c.Default:
			return 1
		}
		return 2
	}
	slices.SortStableFunc(out, func(a, b *Credential) int { return rank(a) - rank(b) })
	return out
}
