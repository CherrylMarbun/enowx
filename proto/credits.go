package proto

import (
	"strings"
	"time"
)

// MethodCreditsGet reports what each agent account signed in on the host
// has left of its plan: CreditsParams → CreditsReport. It answers at once
// from what the daemon last read and starts reading again when that is
// old, or when asked to; poll it to see the new reading.
const MethodCreditsGet = "credits.get"

// CreditsParams asks for a fresh reading with Refresh.
type CreditsParams struct {
	Refresh bool `json:"refresh,omitempty"`
}

// CreditsReport is the daemon's latest reading of the host's accounts.
type CreditsReport struct {
	Accounts []CreditAccount `json:"accounts"`
	Checked  time.Time       `json:"checked,omitzero"` // when the reading finished; zero before the first
	Checking bool            `json:"checking,omitempty"`
}

// CreditAccount is one agent's account and its limits.
type CreditAccount struct {
	Agent   string         `json:"agent"`             // agent preset ID: "claude", "codex", "kiro"
	Account string         `json:"account,omitempty"` // the email or name it is signed in as
	Plan    string         `json:"plan,omitempty"`
	Windows []CreditWindow `json:"windows,omitempty"`
	Note    string         `json:"note,omitempty"`  // extra credits, a managed plan, …
	Error   string         `json:"error,omitempty"` // why there is no reading, said for the user
}

// CreditWindow is one limit of an account: how much of it is used and
// when it starts over.
type CreditWindow struct {
	Label    string    `json:"label"` // "5 hours", "Week", "Month"
	Used     float64   `json:"used"`  // percent of the limit, may pass 100
	ResetsAt time.Time `json:"resetsAt,omitzero"`
	Detail   string    `json:"detail,omitempty"` // "611 of 1000 credits"
}

// Short is the window's label as a chip fits it: "5h", "Wk", "Fable".
func (w CreditWindow) Short() string {
	switch w.Label {
	case "5 hours":
		return "5h"
	case "Week":
		return "Wk"
	case "Month":
		return "Mo"
	}
	if rest, ok := strings.CutPrefix(w.Label, "Week · "); ok {
		return rest
	}
	for _, unit := range []struct{ long, short string }{{" days", "d"}, {" hours", "h"}} {
		if n, ok := strings.CutSuffix(w.Label, unit.long); ok {
			return n + unit.short
		}
	}
	return w.Label
}

// Tightest is the account's most used window, and whether it has one.
func (a CreditAccount) Tightest() (CreditWindow, bool) {
	var best CreditWindow
	found := false
	for _, w := range a.Windows {
		if !found || w.Used > best.Used {
			best, found = w, true
		}
	}
	return best, found
}
