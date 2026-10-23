package credits

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// claudeListed is a limit of Claude's usage answer's list, the limits
// Claude Code itself shows: the session, the week, and the week of one
// model.
type claudeListed struct {
	Kind     string  `json:"kind"`
	Percent  float64 `json:"percent"`
	ResetsAt string  `json:"resets_at"`
	Scope    *struct {
		Model *struct {
			Name string `json:"display_name"`
		} `json:"model"`
	} `json:"scope"`
}

// claudeListedLabel names a listed limit, "" for kinds enowX doesn't know.
func claudeListedLabel(l claudeListed) string {
	switch l.Kind {
	case "session":
		return "5 hours"
	case "weekly_all":
		return "Week"
	case "weekly_scoped":
		if l.Scope != nil && l.Scope.Model != nil && l.Scope.Model.Name != "" {
			return "Week · " + l.Scope.Model.Name
		}
	}
	return ""
}

// claudeAccount fills acc from Claude's usage answer: the limits it lists,
// as Claude Code shows them, else (an older answer) its known limits in
// order. Other entries carry internal names, and aren't shown.
func claudeAccount(acc proto.CreditAccount, answer map[string]json.RawMessage) proto.CreditAccount {
	var listed []claudeListed
	if json.Unmarshal(answer["limits"], &listed) == nil {
		for _, l := range listed {
			label := claudeListedLabel(l)
			if label == "" {
				continue
			}
			w := proto.CreditWindow{Label: label, Used: l.Percent}
			if t, err := time.Parse(time.RFC3339Nano, l.ResetsAt); err == nil {
				w.ResetsAt = t
			}
			acc.Windows = append(acc.Windows, w)
		}
	}
	if len(acc.Windows) > 0 {
		return claudeExtra(acc, answer)
	}
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
	for _, k := range claudeLabels {
		if raw, ok := answer[k.key]; ok {
			add(k.label, raw)
		}
	}
	return claudeExtra(acc, answer)
}

// claudeExtra notes the extra usage beyond the plan, when it is on.
func claudeExtra(acc proto.CreditAccount, answer map[string]json.RawMessage) proto.CreditAccount {
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
