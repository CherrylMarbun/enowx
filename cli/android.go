package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/enowx/enowx/core/android"
	"github.com/enowx/enowx/proto"
)

const androidUsage = `usage: enowx-app android COMMAND
  setup [--tv]                     bring this host a JDK, the SDK, an emulator and a virtual phone
                                   (and a virtual Android TV with --tv)
  status                           the SDK, the virtual phones (AVDs) and the devices adb sees
  start AVD                        start a virtual phone
  stop SERIAL                      stop an emulator
  logcat SERIAL [--package P] [--level W] [--follow]
                                   the device's log, only the app's with --package
  run SERIAL [--command CMD] [--session S]
                                   run the session's app on the device, in a new tab of it
  connect ADDRESS [--pair ADDRESS --code CODE]
                                   a phone over Wi-Fi (Wireless debugging), paired the first time
  catch [-s SERIAL] [--off]        send the device's HTTP through Traffic, its CA trusted

Using a device, as the agent would by hand (-s SERIAL when more than one is online):
  screenshot [-s SERIAL] FILE      save its screen as a PNG
  controls [-s SERIAL]             what is on its screen, controls numbered
  tap [-s SERIAL] REF|X Y          touch a control by its number, or a point
  longpress [-s SERIAL] REF|X Y [--ms 800]
  swipe [-s SERIAL] X1 Y1 X2 Y2 [--ms 300]
  type [-s SERIAL] [--into REF] TEXT
                                   type into the field focused (or REF, tapped first); ASCII
  key [-s SERIAL] NAME|CODE        back, home, recents, enter, delete, power, volume-up…;
                                   a TV's remote: up, down, left, right, ok, menu, play-pause`

// android drives the host's Android devices, as the Android tool does.
// Setting up needs no daemon: the Android tool runs it in a tab of its own.
func (r runner) android(args []string) error {
	if len(args) == 0 {
		return errors.New(androidUsage)
	}
	switch args[0] {
	case "setup":
		if len(args) > 1 && args[1] == "--tv" {
			return android.SetupTV(context.Background(), os.Stdin, r.out)
		}
		return android.Setup(context.Background(), os.Stdin, r.out)
	case "help":
		fmt.Fprintln(r.out, androidUsage)
		return nil
	}
	if r.c == nil {
		c, err := Dial()
		if err != nil {
			return err
		}
		defer c.Close()
		r.c = c
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "status":
		return r.androidStatus()
	case "start":
		if len(rest) != 1 {
			return errors.New("usage: enowx-app android start AVD")
		}
		if err := r.c.AndroidStart(rest[0], true); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "Starting %s: \"enowx-app android status\" shows it once adb sees it.\n", rest[0])
		return nil
	case "stop":
		if len(rest) != 1 {
			return errors.New("usage: enowx-app android stop SERIAL")
		}
		return r.c.AndroidStop(proto.AndroidDeviceParams{Serial: rest[0]})
	case "logcat":
		return r.androidLogcat(rest)
	case "run":
		return r.androidRun(rest)
	case "connect":
		return r.androidConnect(rest)
	case "catch":
		return r.androidCatch(rest)
	case "screenshot", "controls", "snapshot", "tap", "longpress", "swipe", "type", "key":
		return r.androidUse(cmd, rest)
	}
	return errors.New(androidUsage)
}

func (r runner) androidStatus() error {
	st, err := r.c.AndroidStatus()
	if err != nil {
		return err
	}
	if st.SDK == "" {
		fmt.Fprintln(r.out, "No Android SDK on this host: \"enowx-app android setup\" brings one.")
	} else {
		fmt.Fprintf(r.out, "SDK: %s\n", st.SDK)
	}
	if st.Java != "" {
		fmt.Fprintf(r.out, "Java: %s\n", st.Java)
	}
	if st.Problem != "" {
		fmt.Fprintf(r.out, "Problem: %s\n", st.Problem)
	}
	if !st.Accel && st.AccelWhy != "" {
		fmt.Fprintf(r.out, "Emulators would be slow: %s\n", st.AccelWhy)
	}
	fmt.Fprintf(r.out, "Virtual phones: %s\n", orNone(strings.Join(st.AVDs, ", ")))
	if len(st.Devices) == 0 {
		fmt.Fprintln(r.out, "Devices: none")
		return nil
	}
	fmt.Fprintln(r.out, "Devices:")
	for _, d := range st.Devices {
		fmt.Fprintf(r.out, "  %s\t%s\t%s\t%s", d.Serial, d.Kind, d.State, d.Name)
		if d.API != "" {
			fmt.Fprintf(r.out, "\tAPI %s", d.API)
		}
		fmt.Fprintln(r.out)
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func (r runner) androidLogcat(args []string) error {
	fs := flag.NewFlagSet("logcat", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pkg := fs.String("package", "", "")
	level := fs.String("level", "", "")
	follow := fs.Bool("follow", false, "")
	serial, err := parseWithArg(fs, args)
	if err != nil || serial == "" {
		return errors.New("usage: enowx-app android logcat SERIAL [--package P] [--level V|D|I|W|E] [--follow]")
	}
	min := strings.Index("VDIWEF", strings.ToUpper(*level))
	var after int64
	for tries := 0; ; tries++ {
		log, err := r.c.AndroidLogcat(proto.AndroidLogcatParams{Serial: serial, Package: *pkg, After: after})
		if err != nil {
			return err
		}
		for _, l := range log.Lines {
			if min > 0 && strings.Index("VDIWEF", l.Level) < min {
				continue
			}
			fmt.Fprintf(r.out, "%s %s %s: %s\n", l.Time, l.Level, l.Tag, l.Message)
		}
		if log.Last > after {
			after = log.Last
		}
		// The first read starts the device's reader, which needs a moment.
		if !*follow && (after > 0 || tries >= 3) {
			return nil
		}
		time.Sleep(time.Second)
	}
}

func (r runner) androidRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	command := fs.String("command", "", "")
	session := fs.String("session", "self", "")
	serial, err := parseWithArg(fs, args)
	if err != nil || serial == "" {
		return errors.New("usage: enowx-app android run SERIAL [--command CMD] [--session S]")
	}
	s, err := r.session(*session)
	if err != nil {
		return err
	}
	run, err := r.c.AndroidRun(proto.AndroidRunParams{Session: s.ID, Serial: serial, Command: *command})
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "Running %q in tab %s (\"enowx-app read %s\" shows it).\n", run.Command, run.Tab, run.Tab)
	if run.Package != "" {
		fmt.Fprintf(r.out, "Its log: enowx-app android logcat %s --package %s\n", serial, run.Package)
	}
	return nil
}

// parseWithArg parses flags given before or after one positional argument.
func parseWithArg(fs *flag.FlagSet, args []string) (string, error) {
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() == 0 {
		return "", nil
	}
	arg := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return "", err
	}
	return arg, nil
}
