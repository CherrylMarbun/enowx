package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

// browser drives a session's Browser, shown by the app connected to the
// host: an agent opens what it built, reads it, clicks and types in it.
func (r runner) browser(args []string) error {
	fs := flag.NewFlagSet("browser", flag.ContinueOnError)
	session := fs.String("session", "self", "")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil || fs.NArg() == 0 {
		return errors.New("usage: enowx-app browser [--session S] open URL|snapshot|click REF|fill REF TEXT|screenshot FILE|unslop [--pass N|done]")
	}
	s, err := r.session(*session)
	if err != nil {
		return err
	}
	if fs.Arg(0) == "unslop" {
		return r.browserUnslop(s.ID, fs.Args()[1:])
	}
	act := proto.BrowserAction{Session: s.ID, Action: fs.Arg(0)}
	rest := fs.Args()[1:]
	need := func(n int, usage string) error {
		if len(rest) < n {
			return errors.New("usage: enowx-app browser " + usage)
		}
		return nil
	}
	file := ""
	switch act.Action {
	case proto.BrowserOpen:
		if err := need(1, "open URL"); err != nil {
			return err
		}
		act.URL = rest[0]
	case proto.BrowserSnapshot:
	case proto.BrowserClick:
		if err := need(1, "click REF"); err != nil {
			return err
		}
		act.Ref = rest[0]
	case proto.BrowserFill:
		if err := need(2, "fill REF TEXT"); err != nil {
			return err
		}
		act.Ref, act.Text = rest[0], strings.Join(rest[1:], " ")
	case proto.BrowserScreenshot:
		if err := need(1, "screenshot FILE"); err != nil {
			return err
		}
		file = rest[0]
	case proto.BrowserBack, proto.BrowserForward, proto.BrowserReload:
	case proto.BrowserEval:
		if err := need(1, "eval JAVASCRIPT"); err != nil {
			return err
		}
		act.Text = strings.Join(rest, " ")
	case proto.BrowserCapture:
		if len(rest) != 1 || (rest[0] != "on" && rest[0] != "off") {
			return errors.New("usage: enowx-app browser capture on|off")
		}
		act.Text = rest[0]
	default:
		return fmt.Errorf("browser: no action %q: open, snapshot, click, fill, screenshot, back, forward, reload, eval or capture", act.Action)
	}
	res, err := r.c.BrowserAct(act)
	if err != nil {
		return err
	}
	switch {
	case file != "":
		if err := os.WriteFile(file, res.Image, 0o644); err != nil {
			return err
		}
		fmt.Fprintln(r.out, file)
	case act.Action == proto.BrowserEval:
		fmt.Fprintln(r.out, res.Text)
	case act.Action == proto.BrowserCapture:
		if act.Text == "on" {
			fmt.Fprintln(r.out, "The Browser's previews go through Traffic: enowx-app traffic list.")
		} else {
			fmt.Fprintln(r.out, "The Browser's previews go straight again.")
		}
	case res.Text != "":
		fmt.Fprintf(r.out, "%s\n%s\n\n%s", res.Title, res.URL, res.Text)
	default:
		fmt.Fprintln(r.out, res.URL)
	}
	return nil
}

// browserUnslop reads the page in session's Browser for what makes it
// look AI-made; --pass, in a redesign the app started, says the pass the
// agent begins.
func (r runner) browserUnslop(session string, args []string) error {
	fs := flag.NewFlagSet("unslop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pass := fs.String("pass", "", "")
	brief := fs.String("brief", "", "")
	if err := fs.Parse(args); err != nil {
		return errors.New("usage: enowx-app browser unslop [--pass N|done] [--brief TEXT|-]")
	}
	text, err := stdinOr(*brief)
	if err != nil {
		return err
	}
	u, err := r.c.BrowserUnslop(proto.BrowserUnslopParams{Session: session, Pass: *pass, Brief: text})
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "%s\n%s", u.Frame, unslop.Report(u.Score, u.Findings))
	for _, f := range u.Findings {
		if len(f.Targets) > 0 {
			fmt.Fprintf(r.out, "  %s: %s\n", f.Title, strings.Join(f.Targets, ", "))
		}
	}
	return nil
}
