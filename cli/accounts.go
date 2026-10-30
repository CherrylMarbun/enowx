package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/enowx/enowx/proto"
)

const accountsUsage = `usage: enowx-app accounts [list [--agent A] | status | use NAME [--session S] [--agent A2] | priority AGENT NAME… | fallback AGENT --on|--ask|--off [--to AGENT|none] [--return|--no-return] | default AGENT NAME [--project P] | log [--last N]]
(NAME is an account's name, "default" its agent's own sign-in)`

// accountsCmd runs `enowx accounts …`: agents' accounts, the order they
// are used in, falling back at a limit, and switching a session's.
func (r runner) accountsCmd(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list", "ls", "status":
		fs := flag.NewFlagSet("accounts list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		only := fs.String("agent", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return errors.New(accountsUsage)
		}
		return r.accountsList(*only)
	case "use":
		return r.accountsUse(args[1:])
	case "priority":
		if len(args) < 3 {
			return errors.New(accountsUsage)
		}
		return r.accountsPrefs(args[1], func(p *proto.AccountPrefs) {
			p.Order = nil
			for _, n := range args[2:] {
				p.Order = append(p.Order, ownName(n))
			}
		})
	case "fallback":
		if len(args) < 3 {
			return errors.New(accountsUsage)
		}
		return r.accountsPrefs(args[1], func(p *proto.AccountPrefs) {
			for i := 2; i < len(args); i++ {
				switch args[i] {
				case "--on", "--auto":
					p.Fallback = proto.FallbackAuto
				case "--ask":
					p.Fallback = proto.FallbackAsk
				case "--off":
					p.Fallback = proto.FallbackOff
				case "--return":
					p.Return = true
				case "--no-return":
					p.Return = false
				case "--to":
					if i+1 < len(args) {
						i++
						p.ToAgent = strings.TrimPrefix(args[i], "none")
					}
				}
			}
		})
	case "default":
		if len(args) < 3 {
			return errors.New(accountsUsage)
		}
		project := ""
		if len(args) >= 5 && args[3] == "--project" {
			project = args[4]
		}
		return r.accountsPrefs(args[1], func(p *proto.AccountPrefs) {
			if project == "" {
				p.Default = ownName(args[2])
				return
			}
			if p.Projects == nil {
				p.Projects = map[string]string{}
			}
			p.Projects[project] = ownName(args[2])
		})
	case "log":
		fs := flag.NewFlagSet("accounts log", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		last := fs.Int("last", 20, "")
		if err := fs.Parse(args[1:]); err != nil {
			return errors.New(accountsUsage)
		}
		log, err := r.c.AccountLog()
		if err != nil {
			return err
		}
		for i, e := range log {
			if i >= *last {
				break
			}
			fmt.Fprintf(r.out, "%s  %s  %s %s  %s  %s\n", e.Time.Local().Format("Jan 2 15:04"), e.Event, e.Agent, nameOrDefault(e.Account), e.Session, e.Reason)
		}
		return nil
	}
	return errors.New(accountsUsage)
}

// ownName is name as preferences keep it: "" for the agent's own.
func ownName(name string) string {
	if strings.EqualFold(name, "default") || strings.EqualFold(name, "own") {
		return ""
	}
	return name
}

func nameOrDefault(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

func (r runner) accountsList(only string) error {
	info, err := r.c.AccountsInfo()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tACCOUNT\tORDER\tSTATE\tLEFT\tID")
	for _, a := range info.Accounts {
		if only != "" && a.Agent != only {
			continue
		}
		pr := info.Prefs[a.Agent]
		order := slices.Index(pr.Order, a.Name) + 1
		state := a.State
		if !a.Until.IsZero() {
			state += " until " + a.Until.Local().Format("Jan 2 15:04")
		}
		left := "-"
		if c := a.Credits; c != nil {
			var parts []string
			for _, wd := range c.Windows {
				parts = append(parts, fmt.Sprintf("%s %.0f%%", wd.Short(), wd.Used))
			}
			if c.Error != "" {
				parts = append(parts, c.Error)
			}
			left = strings.Join(parts, " ")
		}
		name := nameOrDefault(a.Name)
		if a.Cred != "" {
			name += " (key " + a.Cred + ")"
		}
		ord := "-"
		if order > 0 {
			ord = fmt.Sprint(order)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", a.Agent, name, ord, state, left, a.ID)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	for agentID, pr := range info.Prefs {
		if only != "" && agentID != only {
			continue
		}
		fb := map[string]string{proto.FallbackOff: "off", proto.FallbackAuto: "on", proto.FallbackAsk: "asks first"}[pr.Fallback]
		line := fmt.Sprintf("%s: fallback %s", agentID, fb)
		if pr.ToAgent != "" {
			line += ", then " + pr.ToAgent
		}
		if pr.Return {
			line += ", back to the first once it resets"
		}
		fmt.Fprintln(r.out, line)
	}
	return nil
}

func (r runner) accountsPrefs(agentID string, change func(p *proto.AccountPrefs)) error {
	info, err := r.c.AccountsInfo()
	if err != nil {
		return err
	}
	p := info.Prefs[agentID]
	change(&p)
	_, err = r.c.SetAccountPrefs(agentID, p)
	return err
}

func (r runner) accountsUse(args []string) error {
	fs := flag.NewFlagSet("accounts use", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	session := fs.String("session", os.Getenv(EnvSession), "")
	other := fs.String("agent", "", "")
	name := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil || *session == "" || name == "" && *other == "" {
		return errors.New(accountsUsage)
	}
	s, err := r.session(*session)
	if err != nil {
		return err
	}
	id := ""
	if ownName(name) != "" {
		if id, err = r.account(name); err != nil {
			return err
		}
	}
	info, err := r.c.SwitchAccount(proto.AccountSwitchParams{Session: s.ID, Account: id, Agent: *other, Continue: true})
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "%s goes on as %s (%s)\n", info.ID, nameOrDefault(name), time.Now().Format("15:04"))
	return nil
}

// accountTools are the accounts as MCP tools.
var accountTools = []mcpTool{
	{name: "accounts_list", description: "The agents' accounts on this host: their order, state (a limit reached and until when, out of credits, signed out), what they have left, and the fallback settings.",
		props: map[string]mcpProp{"agent": {"string", "Only this agent's (claude, codex)"}},
		args:  func(a map[string]any) []string { return withFlag([]string{"list"}, a, "agent", "agent") }},
	{name: "accounts_use", description: "Have a session go on as another account (or, with agent, another agent), its conversation resumed, when the user asks.",
		props:    map[string]mcpProp{"account": {"string", "The account's name, default for the agent's own"}, "session": {"string", "The session (default: this one)"}, "agent": {"string", "Another agent to go on with instead"}},
		required: []string{"account"},
		args: func(a map[string]any) []string {
			return withFlag(withFlag([]string{"use", str(a, "account")}, a, "session", "session"), a, "agent", "agent")
		}},
	{name: "accounts_priority", description: "Set the order an agent's accounts are used in, when the user asks.",
		props:    map[string]mcpProp{"agent": {"string", "claude or codex"}, "order": {"string", "Account names, comma-separated, default for the agent's own"}},
		required: []string{"agent", "order"},
		args: func(a map[string]any) []string {
			return append([]string{"priority", str(a, "agent")}, strings.Split(str(a, "order"), ",")...)
		}},
	{name: "accounts_fallback", description: "Turn falling back to the next account at a limit on, off or ask-first for an agent, when the user asks.",
		props:    map[string]mcpProp{"agent": {"string", "claude or codex"}, "mode": {"string", "on, ask or off"}, "to": {"string", "Another agent once every account is spent, or none"}, "return": {"boolean", "Back to the first account once its limit resets"}},
		required: []string{"agent", "mode"},
		args: func(a map[string]any) []string {
			args := []string{"fallback", str(a, "agent"), "--" + str(a, "mode")}
			if to := str(a, "to"); to != "" {
				args = append(args, "--to", to)
			}
			switch str(a, "return") {
			case "true":
				args = append(args, "--return")
			case "false":
				args = append(args, "--no-return")
			}
			return args
		}},
	{name: "accounts_log", description: "Which account each session used, when, and why it fell back.",
		props: map[string]mcpProp{"last": {"number", "How many (20)"}}, args: func(a map[string]any) []string { return withFlag([]string{"log"}, a, "last", "last") }},
}
