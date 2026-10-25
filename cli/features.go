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

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

const featuresUsage = `usage: enowx-app features COMMAND
  (none) | list [--project P]      every feature, on or off here, and what its guide costs
  on|off NAME [--global|--host|--project P|--session [ID]]
                                   turn one on or off (default: everywhere); from an agent,
                                   it asks the user in the app and waits for their answer
  reset NAME --SCOPE               take a scope's switch away, the wider one deciding
  lazy on|off                      long features as a line pointing at "help NAME"
  preset Minimal|"Web dev"|Android|All [--host|--project P]

A feature turned off is left out of what agents are told and the MCP tools
they get, from their next session; "enowx-app help NAME" tells of one`

// features shows and sets which of enowX's features agents get.
func (r runner) features(args []string) error {
	cmd := "list"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "")
	host := fs.Bool("host", false, "")
	global := fs.Bool("global", false, "")
	sessionFlag := fs.Bool("session", false, "")
	reason := fs.String("reason", "", "")
	arg, err := parseWithArg(fs, args)
	if err != nil {
		return errors.New(featuresUsage)
	}
	scope, sess := "global", ""
	switch {
	case *sessionFlag:
		scope, sess = "session", os.Getenv(EnvSession)
		if rest := fs.Args(); len(rest) > 0 {
			sess = rest[0]
		}
	case *project != "":
		scope = "project"
	case *host:
		scope = "host"
	case *global:
		scope = "global"
	}
	switch cmd {
	case "list":
		st, err := r.c.Features(proto.FeaturesParams{Project: *project, Session: os.Getenv(EnvSession)})
		if err != nil {
			return err
		}
		return r.printFeatures(st)
	case "on", "off", "reset":
		if _, ok := agent.FindFeature(arg); !ok {
			return fmt.Errorf("features: no feature %q: enowx-app features lists them", arg)
		}
		on := cmd == "on"
		p := proto.FeatureSetParams{ID: arg, On: &on, Scope: scope, Project: *project, Session: sess}
		if cmd == "reset" {
			p.On = nil
		}
		if os.Getenv(EnvSession) != "" && cmd == "on" {
			// An agent doesn't turn features on by itself: the user does.
			a, err := r.c.RequestFeature(proto.FeatureRequest{ID: arg, Session: os.Getenv(EnvSession), Scope: scope, Reason: *reason})
			if err != nil {
				return err
			}
			if !a.Allowed {
				return fmt.Errorf("features: the user kept %s off", arg)
			}
			fmt.Fprintf(r.out, "%s is on (%s); its tools come with the next session: \"enowx-app help %s\" tells how to use it now\n", arg, a.Scope, arg)
			return nil
		}
		st, err := r.c.SetFeature(p)
		if err != nil {
			return err
		}
		return r.printFeatures(st)
	case "lazy":
		if arg != "on" && arg != "off" {
			return errors.New(featuresUsage)
		}
		st, err := r.c.SetFeaturesLazy(arg == "on")
		if err != nil {
			return err
		}
		return r.printFeatures(st)
	case "preset":
		st, err := r.c.FeaturesPreset(proto.FeaturesPresetParams{Name: arg, Scope: scope, Project: *project})
		if err != nil {
			return err
		}
		return r.printFeatures(st)
	case "help":
		if arg != "" && featureHelp(r.out, arg) {
			return nil
		}
		fmt.Fprintln(r.out, featuresUsage)
		return nil
	}
	return errors.New(featuresUsage)
}

func (r runner) printFeatures(st proto.FeaturesState) error {
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	for _, g := range agent.FeatureGroups {
		for _, f := range st.Features {
			if f.Group != g {
				continue
			}
			state := "off"
			if f.On {
				state = "on"
			}
			if f.Core {
				state = "on (always)"
			} else if f.From != "" {
				state += " (" + f.From + ")"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t~%d tokens\t%s\n", f.ID, state, f.Group, f.Tokens, f.Name)
		}
	}
	w.Flush()
	lazy := "full"
	if st.Lazy {
		lazy = "lazy"
	}
	fmt.Fprintf(r.out, "guide: ~%d tokens (%s; every feature in full: ~%d)\n", st.Tokens, lazy, st.AllCost)
	return nil
}

// featureHelp prints feature id's part of the guide, and whether it is on.
func featureHelp(out io.Writer, id string) bool {
	f, ok := agent.FindFeature(id)
	if !ok {
		return false
	}
	fmt.Fprintf(out, "%s: %s\n\n", f.Name, f.About)
	if f.Guide != "" {
		fmt.Fprintln(out, strings.TrimSpace(f.Guide))
	}
	if c, err := Dial(); err == nil {
		defer c.Close()
		if st, err := c.Features(proto.FeaturesParams{Session: os.Getenv(EnvSession)}); err == nil {
			if i := slices.IndexFunc(st.Features, func(x proto.FeatureInfo) bool { return x.ID == id }); i >= 0 && !st.Features[i].On {
				fmt.Fprintf(out, "\nIt is turned off: \"enowx-app features on %s\" asks the user to turn it on.\n", id)
			}
		}
	}
	return true
}

// featureTools are the switches as MCP tools.
var featureTools = []mcpTool{
	{name: "features_list", description: "enowX's features (Browser, Traffic, Android, Canvas, the vault…), on or off for this session, and what each costs in the guide.",
		args: func(map[string]any) []string { return []string{"list"} }},
	{name: "features_request", description: "Ask the user to turn a feature on that this work needs; waits for their answer in the app. Say why.",
		props: map[string]mcpProp{"name": {"string", "The feature's ID, from features_list"}, "reason": {"string", "Why it's needed, for the user"}}, required: []string{"name"},
		args: func(a map[string]any) []string {
			return withFlag([]string{"on", str(a, "name")}, a, "reason", "reason")
		}},
	{name: "feature_help", description: "How to use a feature (its part of the guide), on or off.",
		props: map[string]mcpProp{"name": {"string", "The feature's ID"}}, required: []string{"name"},
		args: func(a map[string]any) []string { return []string{"help", str(a, "name")} }},
}
