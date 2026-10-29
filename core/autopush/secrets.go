package autopush

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Never committed by an autopush, whatever .gitignore says: files of
// secrets, by their names.
var secretNames = []*regexp.Regexp{
	regexp.MustCompile(`^\.env(\..+)?$`),
	regexp.MustCompile(`(?i)^[^.].*\.env$`), // prod.env
	regexp.MustCompile(`(?i)\.(pem|key|p12|pfx|jks|keystore|ppk|kdbx|tfstate|ovpn)$`),
	regexp.MustCompile(`^id_(rsa|dsa|ecdsa|ed25519)(_[A-Za-z0-9_-]+)?$`),
	regexp.MustCompile(`(?i)^(credentials|secrets?)(\.(json|ya?ml|toml|ini|txt|env|xml|conf|cfg|properties))?$`),
	regexp.MustCompile(`^\.(npmrc|pypirc|netrc|git-credentials|htpasswd|pgpass|dockercfg)$`),
	regexp.MustCompile(`(?i)^kubeconfig([._-].*)?$`),
	regexp.MustCompile(`(?i)service[-_]?account.*\.json$`),
}

// Templates of .env files hold no secrets, by convention.
var envTemplate = regexp.MustCompile(`^\.env\.(example|sample|template|dist)$`)

// SecretFile reports whether the file at p (with slashes) holds secrets
// by its name, and so is never pushed.
func SecretFile(p string) bool {
	name := path.Base(p)
	if envTemplate.MatchString(name) {
		return false
	}
	for _, re := range secretNames {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// secretPatterns are what a secret looks like inside a file.
var secretPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"a private key", regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP |ENCRYPTED )?PRIVATE KEY( BLOCK)?-----`)},
	{"an AWS access key", regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"a GitHub token", regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{60,})\b`)},
	{"a GitLab token", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}`)},
	{"an Anthropic API key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{30,}`)},
	{"an OpenAI API key", regexp.MustCompile(`\bsk-(proj-)?[A-Za-z0-9_-]{32,}`)},
	{"a Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{"a Slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"a Stripe secret key", regexp.MustCompile(`\b[rs]k_live_[0-9A-Za-z]{20,}`)},
	{"a Telegram bot token", regexp.MustCompile(`\b\d{8,10}:AA[0-9A-Za-z_-]{33}\b`)},
	{"a connection string's password", regexp.MustCompile(`\b(postgres(ql)?|mysql|mongodb(\+srv)?|redis|amqp)://[^\s:/@]+:[^\s@/]{3,}@`)},
	{"a secret assigned in code", regexp.MustCompile(`(?i)\b(api[_-]?key|secret[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret|password)\b["']?\s*[:=]\s*["'][A-Za-z0-9+/_\-.]{16,}["']`)},
	{"a JSON Web Token", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)},
	{"an npm token", regexp.MustCompile(`\bnpm_[A-Za-z0-9]{36}\b|_authToken\s*=\s*[^\s$]{16,}`)},
	{"a Hugging Face token", regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}\b`)},
	{"a kubeconfig's credential", regexp.MustCompile(`^\s*(client-key-data|token|auth-provider-token)\s*:\s*["']?[A-Za-z0-9+/=._-]{20,}["']?\s*$`)},
}

// envSecret is a secret set as an environment variable is, unquoted or
// not: a name in capitals that says it is one, then a value.
var envSecret = regexp.MustCompile(`^\s*(?:export\s+)?([A-Z][A-Z0-9_]*(?:SECRET|PASSWORD|PASSWD|TOKEN|API_?KEY|PRIVATE_?KEY|ACCESS_?KEY|CREDENTIALS?)[A-Z0-9_]*)\s*[=:]\s*["']?([^\s"']+)`)

// placeholder is a value that only stands for one.
var placeholder = regexp.MustCompile(`(?i)^(\$|<|\{|%|\[)|your|example|changeme|change_me|placeholder|dummy|redacted|xxxx|\*\*\*|\.\.\.|^(true|false|null|none|empty|secret|password|token)$`)

// envSecretIn says whether line sets an environment variable named as a
// secret to a value that looks like one: 12 characters or more, a digit
// or both cases among them, and no placeholder.
func envSecretIn(line []byte) bool {
	m := envSecret.FindSubmatch(line)
	if m == nil {
		return false
	}
	v := string(m[2])
	if len(v) < 12 || placeholder.MatchString(v) {
		return false
	}
	digit := strings.ContainsAny(v, "0123456789")
	upper, lower := strings.ToLower(v) != v, strings.ToUpper(v) != v
	return digit || upper && lower
}

// binaryScanned is how much of a binary file is looked through for
// secrets.
const binaryScanned = 4 << 20

// Finding is a secret found in a file about to be pushed.
type Finding struct {
	Path string
	Line int // 0 for a file of secrets by its name
	Kind string
}

func (f Finding) String() string {
	if f.Line == 0 {
		return f.Path + ": " + f.Kind
	}
	return fmt.Sprintf("%s:%d: %s", f.Path, f.Line, f.Kind)
}

// ScanSecrets looks for secrets in data, file p's content: by line in
// text, through the first MBs of a binary file (a keystore, an archive)
// for what can't be mistaken.
func ScanSecrets(p string, data []byte) []Finding {
	if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		head := data[:min(len(data), binaryScanned)]
		for _, sp := range secretPatterns {
			if sp.re.Match(head) {
				return []Finding{{Path: p, Kind: sp.kind}}
			}
		}
		return nil
	}
	var out []Finding
	for i, line := range bytes.Split(data, []byte("\n")) {
		if len(line) > 4000 {
			line = line[:4000]
		}
		found := ""
		for _, sp := range secretPatterns {
			if sp.re.Match(line) {
				found = sp.kind
				break
			}
		}
		if found == "" && envSecretIn(line) {
			found = "a secret set in a variable"
		}
		if found != "" {
			out = append(out, Finding{Path: p, Line: i + 1, Kind: found})
		}
	}
	return out
}

// SecretsError is a push refused for the secrets it would carry.
type SecretsError struct{ Findings []Finding }

func (e *SecretsError) Error() string {
	var b strings.Builder
	b.WriteString("autopush: not pushed: secrets found (remove them, or add the files to .gitignore):")
	for i, f := range e.Findings {
		if i == 10 {
			fmt.Fprintf(&b, "\n  and %d more", len(e.Findings)-i)
			break
		}
		b.WriteString("\n  " + f.String())
	}
	return b.String()
}
