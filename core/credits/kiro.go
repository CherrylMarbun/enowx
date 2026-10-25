package credits

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/enowx/enowx/internal/quietcmd"
	"github.com/enowx/enowx/proto"
)

// Kiro CLI prints its usage for /usage, run without a conversation:
//
//	Estimated Usage | resets on 2026-11-01 | KIRO POWER
//	Credits (10663.33 of 10000 covered in plan), 106.6%
var (
	kiroHeader  = regexp.MustCompile(`(?i)resets on (\d{4}-\d{2}-\d{2})\s*\|\s*(.+)`)
	kiroCredits = regexp.MustCompile(`(?i)credits \(([\d.,]+) of ([\d.,]+)[^)]*\),?\s*([\d.]+)%`)
	ansi        = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)
)

// readKiro asks Kiro CLI for the usage of the account it is signed in
// with.
func readKiro(ctx context.Context) []proto.CreditAccount {
	bin, err := exec.LookPath("kiro-cli")
	if err != nil {
		return nil
	}
	acc := proto.CreditAccount{Agent: "kiro", Account: kiroAccount(ctx, bin)}
	cmd := quietcmd.CommandContext(ctx, bin, "chat", "--no-interactive", "/usage")
	cmd.Dir = os.TempDir() // no project's agent settings apply
	out, err := cmd.CombinedOutput()
	text := ansi.ReplaceAllString(string(out), "")
	if parsed, ok := parseKiroUsage(acc, text); ok {
		return []proto.CreditAccount{parsed}
	}
	switch {
	case strings.Contains(strings.ToLower(text), "login") || strings.Contains(strings.ToLower(text), "sign in"):
		acc.Error = "Not signed in: sign in to Kiro CLI"
	case err != nil:
		acc.Error = "Kiro CLI couldn't report its usage"
	default:
		acc.Error = "Kiro CLI's usage report changed shape"
	}
	return []proto.CreditAccount{acc}
}

// parseKiroUsage fills acc from the text of Kiro CLI's /usage.
func parseKiroUsage(acc proto.CreditAccount, text string) (proto.CreditAccount, bool) {
	m := kiroCredits.FindStringSubmatch(text)
	if m == nil {
		return acc, false
	}
	used, _ := strconv.ParseFloat(m[3], 64)
	w := proto.CreditWindow{Label: "Month", Used: used, Detail: fmt.Sprintf("%s of %s credits", m[1], m[2])}
	if h := kiroHeader.FindStringSubmatch(text); h != nil {
		if t, err := time.ParseInLocation(time.DateOnly, h[1], time.Local); err == nil {
			w.ResetsAt = t
		}
		acc.Plan = titleWords(strings.TrimSpace(h[2]))
	}
	acc.Windows = []proto.CreditWindow{w}
	if strings.Contains(strings.ToLower(text), "managed by your organization") {
		acc.Note = "Managed by your organization"
	}
	return acc, true
}

// kiroAccount is the email Kiro CLI says it is signed in as.
func kiroAccount(ctx context.Context, bin string) string {
	out, err := quietcmd.CommandContext(ctx, bin, "whoami", "--format", "json").Output()
	if err != nil {
		return ""
	}
	// Its JSON comes first; profile lines may follow.
	var who struct {
		Email string `json:"email"`
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	_ = dec.Decode(&who)
	return who.Email
}

// titleWords turns "KIRO POWER" into "Kiro Power".
func titleWords(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		words[i] = titleCase(w)
	}
	return strings.Join(words, " ")
}
