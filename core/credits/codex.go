package credits

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

const codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"

// codexAuth is the part of Codex's auth.json the reading needs.
type codexAuth struct {
	Tokens *struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	} `json:"tokens"`
}

type codexWindow struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds int64    `json:"limit_window_seconds"`
	ResetAt            int64    `json:"reset_at"`
}

type codexUsage struct {
	Email     string `json:"email"`
	PlanType  string `json:"plan_type"`
	RateLimit *struct {
		Allowed         bool         `json:"allowed"`
		PrimaryWindow   *codexWindow `json:"primary_window"`
		SecondaryWindow *codexWindow `json:"secondary_window"`
	} `json:"rate_limit"`
	Credits *struct {
		HasCredits bool    `json:"has_credits"`
		Unlimited  bool    `json:"unlimited"`
		Balance    *string `json:"balance"`
	} `json:"credits"`
}

func codexHome() string {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return h
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

// readCodex reads the ChatGPT plan Codex is signed in with. An API key
// login has no plan limits to show.
func readCodex(ctx context.Context) []proto.CreditAccount {
	data, err := os.ReadFile(filepath.Join(codexHome(), "auth.json"))
	if err != nil {
		return nil
	}
	var auth codexAuth
	if json.Unmarshal(data, &auth) != nil || auth.Tokens == nil || auth.Tokens.AccessToken == "" {
		return nil
	}
	acc := proto.CreditAccount{Agent: "codex", Account: jwtEmail(auth.Tokens.IDToken)}
	headers := map[string]string{"Authorization": "Bearer " + auth.Tokens.AccessToken}
	if auth.Tokens.AccountID != "" {
		headers["ChatGPT-Account-Id"] = auth.Tokens.AccountID
	}
	var u codexUsage
	if err := getJSON(ctx, codexUsageURL, headers, &u); err != nil {
		acc.Error = err.Error()
		return []proto.CreditAccount{acc}
	}
	return []proto.CreditAccount{codexAccount(acc, u)}
}

// codexAccount fills acc from Codex's usage answer.
func codexAccount(acc proto.CreditAccount, u codexUsage) proto.CreditAccount {
	if u.Email != "" {
		acc.Account = u.Email
	}
	acc.Plan = titleCase(u.PlanType)
	if rl := u.RateLimit; rl != nil {
		for _, w := range []*codexWindow{rl.PrimaryWindow, rl.SecondaryWindow} {
			if w == nil || w.UsedPercent == nil {
				continue
			}
			cw := proto.CreditWindow{
				Label: windowLabel(time.Duration(w.LimitWindowSeconds) * time.Second),
				Used:  *w.UsedPercent,
			}
			if w.ResetAt > 0 {
				cw.ResetsAt = time.Unix(w.ResetAt, 0)
			}
			acc.Windows = append(acc.Windows, cw)
		}
		if !rl.Allowed {
			acc.Note = "Blocked until the limit resets"
		}
	}
	if cr := u.Credits; cr != nil && cr.HasCredits && acc.Note == "" {
		switch {
		case cr.Unlimited:
			acc.Note = "Unlimited credits"
		case cr.Balance != nil && *cr.Balance != "":
			acc.Note = "Extra credits: " + *cr.Balance
		}
	}
	return acc
}

// jwtEmail reads the email claim of a JWT without checking it: it only
// names the account.
func jwtEmail(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(payload, &claims)
	return claims.Email
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
