package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/notify"
	"github.com/enowx/enowx/proto"
)

const notifyUsage = `usage: enowx-app notify COMMAND
  (none) | status        how the enowX app notifies: on or off, volume, sounds, quiet hours
  sound [done|input|error] [--text T]
                         play a sound in the user's enowX app (done by default)
  set [--on|--off] [--volume 0-100] [--sound KIND=NAME] [--quiet 22:00-07:00|off]
      [--watched on|off] [--banner on|off] [--badge on|off]
                         change them; NAME is a built-in sound, none or custom:FILE
  mute AGENT|SESSION | unmute AGENT|SESSION
                         silence an agent (claude, codex…) or a session, by its ID`

// notify shows and changes how the app notifies the user of agents, and
// plays its sounds.
func (r runner) notify(args []string) error {
	cmd := "status"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	on := fs.Bool("on", false, "")
	off := fs.Bool("off", false, "")
	volume := fs.String("volume", "", "")
	var sounds []string
	fs.Func("sound", "", func(v string) error { sounds = append(sounds, v); return nil })
	quiet := fs.String("quiet", "", "")
	watched := fs.String("watched", "", "")
	banner := fs.String("banner", "", "")
	badge := fs.String("badge", "", "")
	text := fs.String("text", "", "")
	arg, err := parseWithArg(fs, args)
	if err != nil {
		return errors.New(notifyUsage)
	}
	switch cmd {
	case "status":
		st, err := r.c.NotifySettings()
		if err != nil {
			return err
		}
		r.notifyStatus(st)
		return nil
	case "sound":
		kind := arg
		if kind == "" {
			kind = proto.NotifyDone
		}
		return r.c.NotifySound(proto.NotifySoundParams{Kind: kind, Session: os.Getenv("ENOWX_SESSION"), Text: *text})
	case "mute", "unmute":
		if arg == "" {
			return errors.New(notifyUsage)
		}
		st, err := r.c.NotifySettings()
		if err != nil {
			return err
		}
		list := &st.MutedSessions
		if isAgentID(r, arg) {
			list = &st.MutedAgents
		}
		*list = without(*list, arg)
		if cmd == "mute" {
			*list = append(*list, arg)
		}
		_, err = r.c.SetNotifySettings(st)
		return err
	case "set":
		st, err := r.c.NotifySettings()
		if err != nil {
			return err
		}
		if *on && *off {
			return errors.New("notify: --on or --off, not both")
		}
		if *on || *off {
			st.Off = *off
		}
		if *volume != "" {
			v, err := strconv.ParseFloat(strings.TrimSuffix(*volume, "%"), 64)
			if err != nil || v < 0 || v > 100 {
				return fmt.Errorf("notify: --volume %q: a number from 0 to 100", *volume)
			}
			st.Volume = v / 100
		}
		for _, s := range sounds {
			kind, name, ok := strings.Cut(s, "=")
			if !ok {
				kind, name = proto.NotifyDone, s
			}
			if st.Sounds == nil {
				st.Sounds = map[string]string{}
			}
			st.Sounds[kind] = name
		}
		switch q := *quiet; {
		case q == "off":
			st.Quiet = false
		case q != "":
			from, to, ok := strings.Cut(q, "-")
			if !ok {
				return fmt.Errorf("notify: --quiet %q: FROM-TO, like 22:00-07:00, or off", q)
			}
			st.Quiet, st.QuietFrom, st.QuietTo = true, from, to
		}
		for _, f := range []struct {
			v   string
			set func(bool)
		}{{*watched, func(b bool) { st.Watched = b }}, {*banner, func(b bool) { st.NoBanner = !b }}, {*badge, func(b bool) { st.Badge = b }}} {
			switch f.v {
			case "":
			case "on":
				f.set(true)
			case "off":
				f.set(false)
			default:
				return fmt.Errorf("notify: %q: on or off", f.v)
			}
		}
		out, err := r.c.SetNotifySettings(st)
		if err != nil {
			return err
		}
		r.notifyStatus(out)
		return nil
	case "help":
		fmt.Fprintln(r.out, notifyUsage)
		return nil
	}
	return errors.New(notifyUsage)
}

func (r runner) notifyStatus(st proto.NotifySettings) {
	state := "on"
	if st.Off {
		state = "off"
	}
	fmt.Fprintf(r.out, "sounds: %s, volume %.0f%%\n", state, st.Volume*100)
	for _, k := range notify.Kinds {
		fmt.Fprintf(r.out, "  %s: %s\n", k, st.Sounds[k])
	}
	quiet := "off"
	if st.Quiet {
		quiet = st.QuietFrom + "-" + st.QuietTo
	}
	fmt.Fprintf(r.out, "quiet hours: %s\nwhile watching the session: %s\nbanner: %s\nbadge: %s\n", quiet, onOff(st.Watched), onOff(!st.NoBanner), onOff(st.Badge))
	if len(st.MutedAgents)+len(st.MutedSessions) > 0 {
		fmt.Fprintf(r.out, "muted: %s\n", strings.Join(append(append([]string{}, st.MutedAgents...), st.MutedSessions...), ", "))
	}
	fmt.Fprintf(r.out, "sounds built in: %s\n", strings.Join(notify.Builtins(), ", "))
	fmt.Fprintln(r.out, "(sounds play in the enowX app on the user's computer, from whichever host asks)")
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func without(list []string, s string) []string {
	var out []string
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

// isAgentID reports whether id names an agent of the host, not a session.
func isAgentID(r runner, id string) bool {
	agents, err := r.c.Agents()
	if err != nil {
		return false
	}
	for _, a := range agents {
		if a.ID == id {
			return true
		}
	}
	return false
}

// notifyTools are the notifications as MCP tools.
var notifyTools = []mcpTool{
	{name: "notify_sound", description: "Play a sound in the user's enowX app (on their computer, from any host): done when your work is finished, input when you need them, error when something failed. Use it when the user asked to be told, or for a long task's end.",
		props: map[string]mcpProp{"kind": {"string", "done, input or error"}, "text": {"string", "A line the desktop notification says"}},
		args: func(a map[string]any) []string {
			args := []string{"sound"}
			if k := str(a, "kind"); k != "" {
				args = append(args, k)
			}
			return withFlag(args, a, "text", "text")
		}},
	{name: "notify_status", description: "Whether the app's notification sounds are on, their volume, sounds, quiet hours and what is muted: check before relying on a sound.",
		args: func(map[string]any) []string { return []string{"status"} }},
	{name: "notify_set", description: "Change the app's notification sounds, when the user asks: on or off, volume (0-100), a kind's sound (done=chime), quiet hours (22:00-07:00 or off).",
		props: map[string]mcpProp{"on": {"boolean", "Sounds on"}, "off": {"boolean", "Sounds off"}, "volume": {"number", "0 to 100"},
			"sound": {"string", "KIND=NAME: kind done, input or error; NAME a built-in sound, none or custom:FILE"}, "quiet": {"string", "Quiet hours FROM-TO, or off"}},
		args: func(a map[string]any) []string {
			args := []string{"set"}
			for _, k := range []string{"on", "off", "volume", "sound", "quiet"} {
				args = withFlag(args, a, k, k)
			}
			return args
		}},
}
