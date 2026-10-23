package credits

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func TestCodexWindowsAreNamedByLength(t *testing.T) {
	var u codexUsage
	answer := `{"email":"a@b.c","plan_type":"plus","rate_limit":{"allowed":true,
		"primary_window":{"used_percent":82,"limit_window_seconds":18000,"reset_at":1784530149},
		"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_at":1785000000}},
		"credits":{"has_credits":true,"unlimited":false,"balance":"$4.20"}}`
	if err := json.Unmarshal([]byte(answer), &u); err != nil {
		t.Fatal(err)
	}
	acc := codexAccount(proto.CreditAccount{Agent: "codex"}, u)
	if acc.Account != "a@b.c" || acc.Plan != "Plus" || acc.Note != "Extra credits: $4.20" || len(acc.Windows) != 2 {
		t.Fatalf("account = %+v", acc)
	}
	if w := acc.Windows[0]; w.Label != "5 hours" || w.Used != 82 || !w.ResetsAt.Equal(time.Unix(1784530149, 0)) {
		t.Fatalf("first window = %+v", w)
	}
	if acc.Windows[1].Label != "Week" {
		t.Fatalf("second window = %+v", acc.Windows[1])
	}
	if w, _ := acc.Tightest(); w.Used != 82 {
		t.Fatalf("tightest = %+v", w)
	}
}

// The limits Claude lists show as Claude Code shows them; entries with
// internal names don't.
func TestClaudeListedLimits(t *testing.T) {
	var answer map[string]json.RawMessage
	raw := `{"five_hour":{"utilization":8,"resets_at":"2026-10-23T13:59:59.96+00:00"},
		"iguana_necktie":{"utilization":71.2,"resets_at":"2026-11-05T07:59:00+00:00"},
		"limits":[
			{"kind":"session","percent":8,"resets_at":"2026-10-23T13:59:59.961730+00:00","scope":null},
			{"kind":"weekly_all","percent":6,"resets_at":"2026-10-30T03:59:59.961748+00:00","scope":null},
			{"kind":"weekly_scoped","percent":0,"resets_at":"2026-10-30T04:00:00+00:00","scope":{"model":{"id":null,"display_name":"Fable"}}}]}`
	if err := json.Unmarshal([]byte(raw), &answer); err != nil {
		t.Fatal(err)
	}
	acc := claudeAccount(proto.CreditAccount{Agent: "claude"}, answer)
	var labels []string
	for _, w := range acc.Windows {
		labels = append(labels, w.Label)
	}
	if strings.Join(labels, ", ") != "5 hours, Week, Week · Fable" || acc.Windows[0].ResetsAt.IsZero() {
		t.Fatalf("windows = %+v", acc.Windows)
	}
}

func TestClaudeLimitsKeepTheirOrder(t *testing.T) {
	var answer map[string]json.RawMessage
	raw := `{"seven_day":{"utilization":47,"resets_at":"2026-08-13T03:00:00+00:00"},
		"five_hour":{"utilization":4.0,"resets_at":"2026-08-08T11:00:00.943648+00:00"},
		"seven_day_opus":null,"limits":[{"x":1}],"iguana_necktie":{"utilization":71},
		"extra_usage":{"is_enabled":true,"utilization":12}}`
	if err := json.Unmarshal([]byte(raw), &answer); err != nil {
		t.Fatal(err)
	}
	acc := claudeAccount(proto.CreditAccount{Agent: "claude"}, answer)
	if len(acc.Windows) != 2 || acc.Windows[0].Label != "5 hours" || acc.Windows[1].Label != "Week" {
		t.Fatalf("windows = %+v", acc.Windows)
	}
	if acc.Windows[0].ResetsAt.IsZero() || acc.Note != "Extra usage: 12% of the monthly cap" {
		t.Fatalf("account = %+v", acc)
	}
}

func TestKiroUsageText(t *testing.T) {
	text := "Estimated Usage | resets on 2026-11-01 | KIRO POWER\nCredits (10663.33 of 10000 covered in plan), 106.6%\nYour plan is managed by your organization's administrator.\n"
	acc, ok := parseKiroUsage(proto.CreditAccount{Agent: "kiro"}, text)
	if !ok || acc.Plan != "Kiro Power" || acc.Note != "Managed by your organization" {
		t.Fatalf("account = %+v, %v", acc, ok)
	}
	w := acc.Windows[0]
	if w.Used != 106.6 || w.Detail != "10663.33 of 10000 credits" || w.ResetsAt.Format(time.DateOnly) != "2026-11-01" {
		t.Fatalf("window = %+v", w)
	}
	if _, ok := parseKiroUsage(proto.CreditAccount{}, "error: not logged in"); ok {
		t.Fatal("parsed a refusal")
	}
}
