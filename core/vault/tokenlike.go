package vault

import (
	"regexp"
	"strings"
)

// Tokens copied from a platform's page: the app offers one the user
// copied after opening the page where it is made, when it looks like that
// platform's.

// tokenShapes are platforms' tokens by their prefixes and alphabets.
var tokenShapes = map[string]*regexp.Regexp{
	"github":       regexp.MustCompile(`^(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{30,}$|^github_pat_[A-Za-z0-9_]{40,}$`),
	"ghcr":         regexp.MustCompile(`^(ghp|gho)_[A-Za-z0-9]{30,}$|^github_pat_[A-Za-z0-9_]{40,}$`),
	"gitlab":       regexp.MustCompile(`^glpat-[A-Za-z0-9_-]{20,}$`),
	"anthropic":    regexp.MustCompile(`^sk-ant-[A-Za-z0-9_-]{30,}$`),
	"openai":       regexp.MustCompile(`^sk-(proj-|svcacct-)?[A-Za-z0-9_-]{30,}$`),
	"openrouter":   regexp.MustCompile(`^sk-or-v1-[A-Za-z0-9]{40,}$`),
	"gemini":       regexp.MustCompile(`^AIza[0-9A-Za-z_-]{35}$`),
	"stripe":       regexp.MustCompile(`^(sk|rk)_(live|test)_[A-Za-z0-9]{20,}$`),
	"npm":          regexp.MustCompile(`^npm_[A-Za-z0-9]{30,}$`),
	"pypi":         regexp.MustCompile(`^pypi-[A-Za-z0-9_-]{50,}$`),
	"notion":       regexp.MustCompile(`^(ntn|secret)_[A-Za-z0-9]{30,}$`),
	"digitalocean": regexp.MustCompile(`^dop_v1_[a-f0-9]{64}$`),
	"supabase":     regexp.MustCompile(`^sbp_[a-f0-9]{40}$`),
	"sentry":       regexp.MustCompile(`^sntry[su]_[A-Za-z0-9_=+/-]{30,}$`),
	"tailscale":    regexp.MustCompile(`^tskey-[A-Za-z0-9-]{20,}$`),
	"dockerhub":    regexp.MustCompile(`^dckr_pat_[A-Za-z0-9_-]{20,}$`),
	"fly":          regexp.MustCompile(`^(fo1_[A-Za-z0-9_-]{30,}|FlyV1 \S{30,})$`),
	"telegram":     regexp.MustCompile(`^\d{6,12}:[A-Za-z0-9_-]{30,}$`),
}

// anyToken is what a token of a platform without a known shape looks like:
// one word of 20 to 400 token characters.
var anyToken = regexp.MustCompile(`^[A-Za-z0-9_\-.=:+/]{20,400}$`)

// TokenLike reports whether s, as copied, looks like a token of platform
// id: one of its shape when it has one, else any long single word.
func TokenLike(id, s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "\n\t") {
		return false
	}
	if re, ok := tokenShapes[id]; ok {
		return re.MatchString(s)
	}
	return anyToken.MatchString(s) && !strings.Contains(s, "://")
}

// TokenField is the field a copied token goes in: the platform's first
// secret one, "" when it has none.
func TokenField(p Platform) string {
	for _, f := range p.Fields {
		if f.Secret && !f.Multiline {
			return f.Key
		}
	}
	return ""
}
