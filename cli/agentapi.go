package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/enowx/enowx/proto"
)

const apiUsage = `usage: enowx-app api COMMAND
  (no command)                     the agents that take a model API of yours, and what each is set to
  show AGENT                       one agent: its API, the files it is set in, their backups
  set AGENT --url URL [--key KEY | --key-env VAR | --key-stdin] [--model M] [--small-model M] [--protocol P]
                                   point the agent at the API, its settings kept first
                                   (P: anthropic, openai, openai-responses or gemini, as the agent speaks)
  clear AGENT                      back to its own API, the rest of its settings kept
  restore AGENT [BACKUP]           its settings as a backup kept them; without one, as you had them`

// api points the host's agents at a model API of the user's choosing.
func (r runner) api(args []string) error {
	if len(args) == 0 {
		list, err := r.c.AgentAPIs()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "AGENT\tAPI\tMODEL\tKEY\tINSTALLED")
		for _, a := range list {
			api, model, key := "its own", "", ""
			if c := a.Current; c != nil {
				api, model, key = c.BaseURL, c.Model, c.APIKey
			}
			inst := ""
			if a.Installed {
				inst = "yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.Agent, api, orDash(model), orDash(key), inst)
		}
		return w.Flush()
	}
	cmd, rest := args[0], args[1:]
	if cmd == "help" {
		fmt.Fprintln(r.out, apiUsage)
		return nil
	}
	if len(rest) == 0 {
		return errors.New(apiUsage)
	}
	agent := rest[0]
	switch cmd {
	case "show":
		list, err := r.c.AgentAPIs()
		if err != nil {
			return err
		}
		for _, a := range list {
			if a.Agent == agent {
				r.writeAPI(a)
				return nil
			}
		}
		return fmt.Errorf("api: %s takes no API of yours (enowx-app api lists those that do)", agent)
	case "set":
		fs := flagSet("api set")
		url := fs.String("url", "", "")
		key := fs.String("key", "", "")
		keyEnv := fs.String("key-env", "", "")
		keyStdin := fs.Bool("key-stdin", false, "")
		model := fs.String("model", "", "")
		small := fs.String("small-model", "", "")
		protocol := fs.String("protocol", "", "")
		if err := fs.Parse(rest[1:]); err != nil || *url == "" {
			return errors.New(apiUsage)
		}
		k := *key
		switch {
		case *keyEnv != "":
			if k = os.Getenv(*keyEnv); k == "" {
				return fmt.Errorf("api: $%s is empty", *keyEnv)
			}
		case *keyStdin:
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil && line == "" {
				return fmt.Errorf("api: reading the key: %w", err)
			}
			k = strings.TrimSpace(line)
		}
		info, err := r.c.SetAgentAPI(agent, proto.AgentAPI{BaseURL: *url, APIKey: k, Model: *model, SmallModel: *small, Protocol: *protocol})
		if err != nil {
			return err
		}
		r.writeAPI(info)
		fmt.Fprintln(r.out, "\nSessions of it started from now on use it; \"enowx-app api restore "+agent+"\" puts its settings back.")
		return nil
	case "clear":
		info, err := r.c.ClearAgentAPI(agent)
		if err != nil {
			return err
		}
		r.writeAPI(info)
		return nil
	case "restore":
		backup := ""
		if len(rest) > 1 {
			backup = rest[1]
		}
		info, err := r.c.RestoreAgentAPI(agent, backup)
		if err != nil {
			return err
		}
		r.writeAPI(info)
		return nil
	}
	return errors.New(apiUsage)
}

func (r runner) writeAPI(a proto.AgentAPIInfo) {
	fmt.Fprintf(r.out, "%s (%s)\n", a.Name, a.Agent)
	if a.Error != "" {
		fmt.Fprintf(r.out, "  problem: %s\n", a.Error)
	}
	if c := a.Current; c != nil {
		fmt.Fprintf(r.out, "  API      %s (%s)\n  model    %s\n  key      %s\n", c.BaseURL, orDash(c.Protocol), orDash(c.Model), orDash(c.APIKey))
		if c.SmallModel != "" {
			fmt.Fprintf(r.out, "  small    %s\n", c.SmallModel)
		}
	} else {
		fmt.Fprintln(r.out, "  API      its own")
	}
	fmt.Fprintf(r.out, "  speaks   %s\n  files    %s\n", strings.Join(a.Protocols, ", "), strings.Join(a.Files, ", "))
	if a.Note != "" {
		fmt.Fprintf(r.out, "  note     %s\n", a.Note)
	}
	for _, b := range a.Backups {
		what := b.Note
		if b.Original {
			what = "as you had it"
		}
		fmt.Fprintf(r.out, "  backup   %s  %s  %s\n", b.ID, b.Time.Format("2006-01-02 15:04"), what)
	}
}
