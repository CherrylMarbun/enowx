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
	regexp.MustCompile(`(?i)\.(pem|key|p12|pfx|jks|keystore|ppk)$`),
	regexp.MustCompile(`^id_(rsa|dsa|ecdsa|ed25519)$`),
	regexp.MustCompile(`(?i)^(credentials|secrets?)(\.[a-z]+)?$`),
	regexp.MustCompile(`^\.(npmrc|pypirc|netrc|git-credentials)$`),
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
}

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

// ScanSecrets looks for secrets in data, file p's content.
func ScanSecrets(p string, data []byte) []Finding {
	if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		return nil // binary
	}
	var out []Finding
	for i, line := range bytes.Split(data, []byte("\n")) {
		if len(line) > 4000 {
			line = line[:4000]
		}
		for _, sp := range secretPatterns {
			if sp.re.Match(line) {
				out = append(out, Finding{Path: p, Line: i + 1, Kind: sp.kind})
				break
			}
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
