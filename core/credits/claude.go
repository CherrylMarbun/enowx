package credits

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

const claudeUsageURL = "https://api.anthropic.com/api/oauth/usage"

// claudeCreds is Claude Code's login, as its keychain item or
// .credentials.json holds it.
type claudeCreds struct {
	OAuth *struct {
		AccessToken      string `json:"accessToken"`
		ExpiresAt        int64  `json:"expiresAt"` // Unix milliseconds
		SubscriptionType string `json:"subscriptionType"`
	} `json:"claudeAiOauth"`
}

// claudeLimit is one limit of Claude's usage answer.
type claudeLimit struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

// claudeLabels names Claude's limits, in the order they show.
var claudeLabels = []struct{ key, label string }{
	{"five_hour", "5 hours"},
	{"seven_day", "Week"},
	{"seven_day_opus", "Week · Opus"},
	{"seven_day_sonnet", "Week · Sonnet"},
}

func claudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// readClaude reads the plan of every Claude Code login on the machine. A
// login that has expired is only shown when there is no live one: old
// configuration folders leave theirs behind.
func readClaude(ctx context.Context) []proto.CreditAccount {
	raws := claudeKeychain(ctx)
	if file, err := os.ReadFile(filepath.Join(claudeDir(), ".credentials.json")); err == nil {
		raws = append(raws, file)
	}
	var live []proto.CreditAccount
	expired := false
	seen := map[string]bool{}
	for _, raw := range raws {
		var creds claudeCreds
		if json.Unmarshal(raw, &creds) != nil || creds.OAuth == nil || creds.OAuth.AccessToken == "" {
			continue
		}
		if seen[creds.OAuth.AccessToken] {
			continue
		}
		seen[creds.OAuth.AccessToken] = true
		if exp := creds.OAuth.ExpiresAt; exp > 0 && time.Now().After(time.UnixMilli(exp)) {
			expired = true
			continue
		}
		live = append(live, readClaudeLogin(ctx, creds))
	}
	if len(live) == 0 && expired {
		return []proto.CreditAccount{{Agent: "claude", Account: claudeEmail(), Error: errSignedOut.Error()}}
	}
	if len(live) == 1 && live[0].Account == "" {
		live[0].Account = claudeEmail()
	}
	return live
}

// readClaudeLogin asks Anthropic for the limits of one login.
func readClaudeLogin(ctx context.Context, creds claudeCreds) proto.CreditAccount {
	acc := proto.CreditAccount{Agent: "claude", Plan: titleCase(creds.OAuth.SubscriptionType)}
	var answer map[string]json.RawMessage
	err := getJSON(ctx, claudeUsageURL, map[string]string{
		"Authorization":  "Bearer " + creds.OAuth.AccessToken,
		"anthropic-beta": "oauth-2025-04-20",
	}, &answer)
	if err != nil {
		acc.Error = err.Error()
		return acc
	}
	return claudeAccount(acc, answer)
}

// claudeAccount fills acc from Claude's usage answer: its known limits in
// order, then any other it reports.
func claudeAccount(acc proto.CreditAccount, answer map[string]json.RawMessage) proto.CreditAccount {
	add := func(label string, raw json.RawMessage) {
		var l claudeLimit
		if json.Unmarshal(raw, &l) != nil || l.Utilization == nil {
			return
		}
		w := proto.CreditWindow{Label: label, Used: *l.Utilization}
		if t, err := time.Parse(time.RFC3339Nano, l.ResetsAt); err == nil {
			w.ResetsAt = t
		}
		acc.Windows = append(acc.Windows, w)
	}
	seen := map[string]bool{"extra_usage": true}
	for _, k := range claudeLabels {
		seen[k.key] = true
		if raw, ok := answer[k.key]; ok {
			add(k.label, raw)
		}
	}
	var rest []string
	for k := range answer {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	slices.Sort(rest)
	for _, k := range rest {
		add(strings.ReplaceAll(titleCase(k), "_", " "), answer[k])
	}
	var extra struct {
		Enabled     bool     `json:"is_enabled"`
		Utilization *float64 `json:"utilization"`
	}
	if raw, ok := answer["extra_usage"]; ok && json.Unmarshal(raw, &extra) == nil && extra.Enabled {
		acc.Note = "Extra usage on"
		if extra.Utilization != nil {
			acc.Note = fmt.Sprintf("Extra usage: %.0f%% of the monthly cap", *extra.Utilization)
		}
	}
	return acc
}

// claudeEmail is the account Claude Code says it is signed in as.
func claudeEmail() string {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude.json")
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		path = filepath.Join(d, ".claude.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg struct {
		OAuthAccount struct {
			Email string `json:"emailAddress"`
			Org   string `json:"organizationName"`
		} `json:"oauthAccount"`
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg.OAuthAccount.Email
}
