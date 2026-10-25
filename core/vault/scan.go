package vault

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/enowx/enowx/proto"
)

// Runner runs a program and returns what it printed, for logins a tool
// keeps in the system's keychain (gh auth token). Nil runs nothing.
type Runner func(name string, args ...string) (string, error)

// Scan finds the logins the tools of the home at home keep, with their
// values, for the user to import. It only reads.
func Scan(home string, run Runner) []proto.VaultFound {
	var out []proto.VaultFound
	add := func(f proto.VaultFound) {
		for k, v := range f.Fields {
			if v = strings.TrimSpace(v); v == "" {
				delete(f.Fields, k)
			} else {
				f.Fields[k] = v
			}
		}
		if len(f.Fields) == 0 {
			return
		}
		if f.Name == "" {
			f.Name = defaultName(Credential{Platform: f.Platform, Account: f.Account})
		}
		if f.Type == "" {
			f.Type = Lookup(f.Platform).Type
		}
		out = append(out, f)
	}
	p := func(rel ...string) string { return filepath.Join(append([]string{home}, rel...)...) }
	scanGH(p(".config", "gh", "hosts.yml"), run, add)
	scanGitCredentials(p(".git-credentials"), add)
	scanNetrc(p(".netrc"), add)
	scanAWS(p(".aws", "credentials"), p(".aws", "config"), add)
	for _, f := range []string{p(".wrangler", "config", "default.toml"), p("Library", "Preferences", ".wrangler", "config", "default.toml"), p(".config", ".wrangler", "config", "default.toml")} {
		scanWrangler(f, add)
	}
	scanDocker(p(".docker", "config.json"), add)
	scanNpmrc(p(".npmrc"), add)
	scanSSH(p(".ssh"), add)
	scanSimple(home, add)
	return out
}

func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

// keyValue reads "key: value", "key = value" or "key=value", quotes off.
func keyValue(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	i := strings.IndexAny(line, ":=")
	if i <= 0 {
		return "", "", false
	}
	k, v := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	v = strings.Trim(v, `"'`)
	return k, v, true
}

// scanGH reads gh's hosts.yml; a token gh keeps in the keychain comes
// from gh auth token.
func scanGH(path string, run Runner, add func(proto.VaultFound)) {
	host, user, token := "", "", ""
	flush := func() {
		if host == "" {
			return
		}
		if token == "" && run != nil {
			if out, err := run("gh", "auth", "token", "--hostname", host); err == nil {
				token = strings.TrimSpace(out)
			}
		}
		platform := "github"
		f := proto.VaultFound{Platform: platform, Account: user, Fields: map[string]string{"token": token}, Source: path}
		if host != "github.com" {
			f.Platform, f.Note = "gitea", "GitHub Enterprise "+host
			f.Fields["url"] = "https://" + host
		}
		add(f)
	}
	for _, line := range readLines(path) {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			flush()
			host, user, token = strings.TrimSuffix(strings.TrimSpace(line), ":"), "", ""
			continue
		}
		if k, v, ok := keyValue(line); ok {
			switch k {
			case "oauth_token":
				token = v
			case "user":
				user = v
			}
		}
	}
	flush()
}

// gitPlatform is the platform of a git server.
func gitPlatform(host string) string {
	switch strings.ToLower(host) {
	case "github.com":
		return "github"
	case "gitlab.com":
		return "gitlab"
	case "bitbucket.org":
		return "bitbucket"
	}
	return ""
}

func loginFound(host, user, password, source string) proto.VaultFound {
	switch gitPlatform(host) {
	case "github":
		return proto.VaultFound{Platform: "github", Account: user, Fields: map[string]string{"token": password}, Source: source}
	case "gitlab":
		return proto.VaultFound{Platform: "gitlab", Account: user, Fields: map[string]string{"token": password}, Source: source}
	case "bitbucket":
		return proto.VaultFound{Platform: "bitbucket", Account: user, Fields: map[string]string{"username": user, "password": password}, Source: source}
	}
	return proto.VaultFound{Platform: "login", Account: user + "@" + host, Fields: map[string]string{"username": user, "password": password}, Source: source, Note: host}
}

func scanGitCredentials(path string, add func(proto.VaultFound)) {
	for _, line := range readLines(path) {
		u, err := url.Parse(strings.TrimSpace(line))
		if err != nil || u.User == nil {
			continue
		}
		pw, _ := u.User.Password()
		add(loginFound(u.Host, u.User.Username(), pw, path))
	}
}

func scanNetrc(path string, add func(proto.VaultFound)) {
	var fields []string
	for _, line := range readLines(path) {
		fields = append(fields, strings.Fields(line)...)
	}
	machine, login, password := "", "", ""
	flush := func() {
		if machine != "" && password != "" {
			add(loginFound(machine, login, password, path))
		}
		machine, login, password = "", "", ""
	}
	for i := 0; i+1 < len(fields); i++ {
		switch fields[i] {
		case "machine":
			flush()
			machine = fields[i+1]
		case "login":
			login = fields[i+1]
		case "password":
			password = fields[i+1]
		default:
			continue
		}
		i++
	}
	flush()
}

// iniSections reads an INI file's sections, "[profile x]" as "x".
func iniSections(path string) map[string]map[string]string {
	out := map[string]map[string]string{}
	section := ""
	for _, line := range readLines(path) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimPrefix(strings.Trim(line, "[]"), "profile ")
			section = strings.Trim(strings.TrimSpace(section), `"`)
			if out[section] == nil {
				out[section] = map[string]string{}
			}
			continue
		}
		if k, v, ok := keyValue(line); ok && section != "" {
			out[section][k] = v
		}
	}
	return out
}

func scanAWS(credentials, config string, add func(proto.VaultFound)) {
	conf := iniSections(config)
	for profile, kv := range iniSections(credentials) {
		f := proto.VaultFound{Platform: "aws", Account: profile, Source: credentials, Fields: map[string]string{
			"access_key_id": kv["aws_access_key_id"], "secret_access_key": kv["aws_secret_access_key"],
			"session_token": kv["aws_session_token"], "region": conf[profile]["region"],
		}}
		if f.Fields["secret_access_key"] == "" {
			continue
		}
		add(f)
	}
}

func scanWrangler(path string, add func(proto.VaultFound)) {
	kv := map[string]string{}
	for _, line := range readLines(path) {
		if k, v, ok := keyValue(line); ok {
			kv[k] = v
		}
	}
	if kv["api_token"] != "" {
		add(proto.VaultFound{Platform: "cloudflare", Fields: map[string]string{"token": kv["api_token"]}, Source: path})
	} else if kv["oauth_token"] != "" {
		add(proto.VaultFound{Platform: "cloudflare", Fields: map[string]string{"token": kv["oauth_token"]}, Source: path,
			Note: "wrangler's OAuth token expires within hours: an API token lasts"})
	}
}

func scanDocker(path string, add func(proto.VaultFound)) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var conf struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if json.Unmarshal(data, &conf) != nil {
		return
	}
	for reg, a := range conf.Auths {
		raw, err := base64.StdEncoding.DecodeString(a.Auth)
		if err != nil {
			continue
		}
		user, pw, ok := strings.Cut(string(raw), ":")
		if !ok {
			continue
		}
		f := proto.VaultFound{Platform: "dockerhub", Account: user, Fields: map[string]string{"username": user, "password": pw}, Source: path}
		switch registryHost(reg) {
		case "ghcr.io":
			f.Platform = "ghcr"
		case "index.docker.io", "docker.io", "registry-1.docker.io":
		default:
			f.Platform, f.Note = "login", "registry "+reg
		}
		add(f)
	}
}

func scanNpmrc(path string, add func(proto.VaultFound)) {
	for _, line := range readLines(path) {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && strings.HasSuffix(k, ":_authToken") {
			f := proto.VaultFound{Platform: "npm", Fields: map[string]string{"token": v}, Source: path}
			if reg := strings.TrimSuffix(strings.TrimPrefix(k, "//"), "/:_authToken"); reg != "registry.npmjs.org" {
				f.Note = "registry " + reg
			}
			add(f)
		}
	}
}

// scanSSH finds the private keys without a passphrase.
func scanSSH(dir string, add func(proto.VaultFound)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "id_") || strings.HasSuffix(name, ".pub") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || len(data) > 64<<10 {
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			continue // a passphrase, or not a key
		}
		pub := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
		add(proto.VaultFound{Name: "ssh-" + strings.TrimPrefix(name, "id_"), Platform: "ssh", Account: name,
			Fields: map[string]string{"private_key": string(data), "public_key": pub}, Source: filepath.Join(dir, name)})
	}
}
