package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/enowx/enowx/proto"
)

// The commands an agent uses a device with: what its screen shows, and
// input, by adb on the device's host.

// online is device serial if given, else the only device online.
func (r runner) online(serial string) (string, error) {
	if serial != "" {
		return serial, nil
	}
	st, err := r.c.AndroidStatus()
	if err != nil {
		return "", err
	}
	var on []string
	for _, d := range st.Devices {
		if d.State == "device" {
			on = append(on, d.Serial)
		}
	}
	switch len(on) {
	case 0:
		return "", errors.New("android: no device online (enowx-app android status)")
	case 1:
		return on[0], nil
	}
	return "", fmt.Errorf("android: %d devices online: say which with -s (%s)", len(on), strings.Join(on, ", "))
}

func (r runner) androidUse(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	serialFlag := fs.String("s", "", "")
	ms := fs.Int("ms", 0, "")
	into := fs.Int("into", 0, "")
	var rest []string
	for len(args) > 0 { // flags anywhere among the arguments
		if err := fs.Parse(args); err != nil {
			return errors.New(androidUsage)
		}
		if fs.NArg() == 0 {
			break
		}
		rest, args = append(rest, fs.Arg(0)), fs.Args()[1:]
	}
	serial, err := r.online(*serialFlag)
	if err != nil {
		return err
	}
	nums := func(n int) ([]int, error) {
		if len(rest) != n {
			return nil, errors.New(androidUsage)
		}
		out := make([]int, n)
		for i, a := range rest {
			v, err := strconv.Atoi(a)
			if err != nil {
				return nil, fmt.Errorf("android: %q isn't a number", a)
			}
			out[i] = v
		}
		return out, nil
	}
	act := proto.AndroidAct{Serial: serial, Ms: *ms}
	switch cmd {
	case "screenshot":
		if len(rest) != 1 {
			return errors.New("usage: enowx-app android screenshot [-s SERIAL] FILE")
		}
		png, err := r.c.AndroidScreenshot(serial)
		if err != nil {
			return err
		}
		if err := os.WriteFile(rest[0], png, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "Saved %s's screen to %s.\n", serial, rest[0])
		return nil
	case "controls", "snapshot":
		ctl, err := r.c.AndroidControls(serial)
		if err != nil {
			return err
		}
		fmt.Fprint(r.out, ctl.Text)
		return nil
	case "tap", "longpress":
		act.Action = proto.AndroidTap
		if cmd == "longpress" {
			act.Action = proto.AndroidLongPress
		}
		if len(rest) == 1 {
			n, err := nums(1)
			if err != nil {
				return err
			}
			act.Ref = n[0]
		} else {
			n, err := nums(2)
			if err != nil {
				return err
			}
			act.X, act.Y = n[0], n[1]
		}
	case "swipe":
		n, err := nums(4)
		if err != nil {
			return err
		}
		act.Action, act.X, act.Y, act.X2, act.Y2 = proto.AndroidSwipe, n[0], n[1], n[2], n[3]
	case "type":
		if len(rest) == 0 {
			return errors.New("usage: enowx-app android type [-s SERIAL] [--into REF] TEXT")
		}
		act.Action, act.Ref, act.Text = proto.AndroidType, *into, strings.Join(rest, " ")
	case "key":
		if len(rest) != 1 {
			return errors.New("usage: enowx-app android key [-s SERIAL] NAME|CODE")
		}
		act.Action, act.Key = proto.AndroidPress, rest[0]
	}
	if err := r.c.AndroidAct(act); err != nil {
		return err
	}
	fmt.Fprintln(r.out, "Done. \"enowx-app android controls\" shows the screen now.")
	return nil
}

func (r runner) androidConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pair := fs.String("pair", "", "")
	code := fs.String("code", "", "")
	addr, err := parseWithArg(fs, args)
	if err != nil || addr == "" {
		return errors.New("usage: enowx-app android connect ADDRESS [--pair ADDRESS --code CODE]")
	}
	if err := r.c.AndroidConnect(proto.AndroidConnectParams{Address: addr, PairAddress: *pair, Code: *code}); err != nil {
		return err
	}
	fmt.Fprintf(r.out, "Connected to %s: \"enowx-app android status\" lists it once adb sees it.\n", addr)
	return nil
}

// androidCatch sends a device's HTTP through this host's interceptor, as
// a source of the session: its proxy and its trust set by adb.
func (r runner) androidCatch(args []string) error {
	fs := flag.NewFlagSet("catch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	serialFlag := fs.String("s", "", "")
	off := fs.Bool("off", false, "")
	session := fs.String("session", "self", "")
	if err := fs.Parse(args); err != nil {
		return errors.New("usage: enowx-app android catch [-s SERIAL] [--off]")
	}
	serial, err := r.online(*serialFlag)
	if err != nil {
		return err
	}
	targets, err := r.c.TrafficAttached()
	if err != nil {
		return err
	}
	var caught *proto.TrafficTarget
	for i, t := range targets {
		if t.Kind == proto.TargetAndroid && t.Device == serial {
			caught = &targets[i]
		}
	}
	if *off {
		if caught == nil {
			return fmt.Errorf("android: %s's traffic isn't caught", serial)
		}
		if _, err := r.c.AndroidProxy(proto.AndroidProxyParams{Serial: serial, Port: portOf(caught.Proxy), Off: true}); err != nil {
			return err
		}
		if err := r.c.TrafficDetach(caught.Source); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "%s's traffic goes straight again.\n", serial)
		return nil
	}
	if caught == nil {
		s, err := r.session(*session)
		if err != nil {
			return err
		}
		t, err := r.c.TrafficAttach(proto.TrafficAttachParams{Kind: proto.TargetAndroid, Session: s.ID, Device: serial, Label: serial})
		if err != nil {
			return err
		}
		caught = &t
	}
	ca, err := r.c.TrafficCA()
	if err != nil {
		return err
	}
	res, err := r.c.AndroidProxy(proto.AndroidProxyParams{Serial: serial, Port: portOf(caught.Proxy), CA: ca.PEM})
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "%s's HTTP goes through Traffic (enowx-app traffic list --all). %s\n", serial, res.Note)
	return nil
}

// portOf is the port of a source's proxy address.
func portOf(proxy string) int {
	u, err := url.Parse(proxy)
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(u.Port())
	return p
}
