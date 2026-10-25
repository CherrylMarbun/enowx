// Package androidview is the side panel's Android tool: the devices of
// the session's host and of this machine, the session's app run on one,
// its screen shown here, and its log (docs/plans/mobile.md).
package androidview

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/android"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// refreshEvery is how often the devices are asked again while the tool
// shows; logEvery how often the log.
const (
	refreshEvery = 3 * time.Second
	logEvery     = 800 * time.Millisecond
	logKept      = 3000
)

// Host is a host whose devices the tool shows.
type Host struct {
	ID, Name string
	Client   *client.Client
	// Session is true for the session's host, where its app runs.
	Session bool
	// Local is this machine, where windows and scrcpy show.
	Local bool
}

// Options connect the tool to the app.
type Options struct {
	Hosts   []Host
	Session proto.SessionInfo
	Update  func(func())
	// OpenTab shows a tab the tool made (setup, the app running).
	OpenTab func(id string)
	// ForwardPort forwards a port of the session's host to this machine,
	// for a device here to reach the session's dev server.
	ForwardPort func(port int)
	// Local is this machine's daemon, connected when a device of another
	// host's screen shows here; nil when none can be.
	Local func() (*client.Client, error)
}

// View is the tool. It belongs to the main thread; what it asks hosts
// reports through Options.Update.
type View struct {
	status   map[string]proto.AndroidStatus // by host ID
	errs     map[string]string
	polling  bool
	wanted   func() bool
	picked   string // host ID + "/" + serial, or + "/avd:" + name
	project  proto.AndroidProject
	projDir  string
	command  string
	notice   string
	busy     string
	log      []proto.AndroidLogLine
	logAfter int64
	logFor   string
	logging  bool
	appOnly  bool
	level    int
	search   string
	logList  ui.ListState
	connect  connectForm
	setup    *localSetup
	bridges  map[string]*client.ADBBridge // by picked device, for runs across hosts
	screens  map[string]*exec.Cmd
	caught   map[string]*caught // devices whose traffic the session's host catches
}

// caught is a device sending its HTTP through the session's host's
// interceptor: the source it is, the port of its host it sends to, and
// the forward that port is when its host is another.
type caught struct {
	source string
	port   int
	ln     net.Listener
	host   Host
	serial string
}

type connectForm struct {
	open                bool
	address, pair, code string
}

// New returns a tool with nothing read.
func New() *View {
	v := &View{status: map[string]proto.AndroidStatus{}, errs: map[string]string{}, appOnly: true,
		bridges: map[string]*client.ADBBridge{}, screens: map[string]*exec.Cmd{}, caught: map[string]*caught{}}
	v.logList.FollowEnd = true
	return v
}

// Close stops the bridges and the screens the tool started.
func (v *View) Close() {
	for _, b := range v.bridges {
		b.Close()
	}
	for _, c := range v.screens {
		if c.Process != nil {
			c.Process.Kill()
		}
	}
	for _, c := range v.caught {
		if c.ln != nil {
			c.ln.Close()
		}
	}
}

// follow asks the hosts for their devices while the tool shows.
func (v *View) follow(o Options, wanted func() bool) {
	v.wanted = wanted
	if v.polling {
		return
	}
	v.polling = true
	go func() {
		for {
			type got struct {
				st  proto.AndroidStatus
				err error
			}
			res := map[string]got{}
			for _, h := range o.Hosts {
				if h.Client == nil {
					continue
				}
				st, err := h.Client.AndroidStatus()
				res[h.ID] = got{st, err}
			}
			keep := make(chan bool, 1)
			o.Update(func() {
				if v.wanted == nil || !v.wanted() {
					v.polling = false
					keep <- false
					return
				}
				keep <- true
				for id, r := range res {
					if r.err != nil {
						v.errs[id] = client.Explain(r.err)
						continue
					}
					delete(v.errs, id)
					v.status[id] = r.st
				}
			})
			if !<-keep {
				return
			}
			time.Sleep(refreshEvery)
		}
	}()
}

// device is a device of the list: its host and itself.
type device struct {
	host Host
	dev  proto.AndroidDevice
	avd  string // a virtual device not running
}

func (d device) key() string {
	if d.avd != "" {
		return d.host.ID + "/avd:" + d.avd
	}
	return d.host.ID + "/" + d.dev.Serial
}

func (d device) label() string {
	name := d.dev.Name
	if d.avd != "" {
		name = d.avd
	}
	return name + " · " + d.host.Name
}

// devices lists every host's devices, then its virtual devices not
// running.
func (v *View) devices(o Options) []device {
	var out []device
	for _, h := range o.Hosts {
		st := v.status[h.ID]
		running := map[string]bool{}
		for _, d := range st.Devices {
			if d.Serial == "" { // starting
				running[d.Name] = true
				out = append(out, device{host: h, dev: d, avd: d.Name})
				continue
			}
			if d.Kind == "emulator" {
				running[d.Name] = true
			}
			out = append(out, device{host: h, dev: d})
		}
		for _, avd := range st.AVDs {
			if !running[avd] {
				out = append(out, device{host: h, avd: avd, dev: proto.AndroidDevice{Kind: "emulator", Name: avd, State: "stopped"}})
			}
		}
	}
	return out
}

func (v *View) pickedDevice(o Options) (device, bool) {
	devs := v.devices(o)
	if v.picked == "" {
		// The first online device, else the first one.
		for _, d := range devs {
			if d.dev.State == "device" {
				v.picked = d.key()
				return d, true
			}
		}
		if len(devs) > 0 {
			v.picked = devs[0].key()
			return devs[0], true
		}
		return device{}, false
	}
	for _, d := range devs {
		if d.key() == v.picked {
			return d, true
		}
	}
	return device{}, false
}

// Draw shows the tool.
func (v *View) Draw(c *ui.Context, o Options, wanted func() bool) {
	v.follow(o, wanted)
	v.readProject(o)
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		if v.notice != "" {
			ui.Row(c).FillWidth().Padding(6, 12).Gap(8).Background(theme.Sunken).Children(func() {
				ui.Text(c, v.notice).FontSize(11.5).TextColor(theme.TextSoft).Grow(1).Shrink(1)
				if widget.IconButton(c, icon.Close, "Dismiss", false).Clicked() {
					v.notice = ""
				}
			})
		}
		if v.setup != nil {
			v.setupView(c, o)
			return
		}
		ui.Column(c).FillWidth().Shrink(0).Padding(10, 12).Gap(10).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
			v.hostNotes(c, o)
			v.deviceRow(c, o)
			v.runRow(c, o)
			if v.connect.open {
				v.connectView(c, o)
			}
		})
		v.logView(c, o)
	})
}

// hostNotes says what a host lacks: the SDK, acceleration.
func (v *View) hostNotes(c *ui.Context, o Options) {
	for _, h := range o.Hosts {
		if err := v.errs[h.ID]; err != "" {
			ui.Text(c.Key("e"+h.ID), h.Name+": "+err).FontSize(11.5).TextColor(theme.Error)
			continue
		}
		st, ok := v.status[h.ID]
		if !ok {
			continue
		}
		switch {
		case st.SDK == "" || !st.ADB || !st.Emulator:
			ui.Row(c.Key("s" + h.ID)).FillWidth().Gap(8).Children(func() {
				ui.Text(c, h.Name+" has no Android SDK yet.").FontSize(11.5).TextColor(theme.Muted).Grow(1).Shrink(1)
				if widget.Button(c, widget.Secondary, icon.Android, "Set up Android", "").Disabled(v.busy != "").Clicked() {
					v.startSetup(o, h)
				}
			})
		case !st.Accel && len(st.AVDs) > 0:
			ui.Text(c.Key("a"+h.ID), h.Name+"'s emulator is slow: "+st.AccelWhy).FontSize(11).TextColor(theme.Warning)
		}
	}
}

func (v *View) deviceRow(c *ui.Context, o Options) {
	devs := v.devices(o)
	d, ok := v.pickedDevice(o)
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		labels := make([]string, len(devs))
		cur := -1
		for i, x := range devs {
			labels[i] = x.label()
			if x.key() == v.picked {
				cur = i
			}
		}
		if len(devs) == 0 {
			ui.Text(c, "No devices: start an emulator after Set up Android, or connect a phone.").FontSize(12).TextColor(theme.Muted).Grow(1).Shrink(1)
		} else {
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				for i, label := range labels {
					x := devs[i]
					row := ui.ButtonBase(c.Key(x.key())).FillWidth().Height(30).PaddingX(10).Gap(8).Radius(theme.RadiusControl).Justify(ui.Start).Label(label).Cursor(ui.CursorPointer)
					if i == cur {
						row.Background(theme.Selected)
					}
					row.Children(func() {
						col := theme.Idle
						switch x.dev.State {
						case "device":
							col = theme.Done
						case "starting":
							col = theme.Working
						case "unauthorized", "offline":
							col = theme.Warning
						}
						widget.Dot(c, col, x.dev.State != "device", 7)
						ui.Text(c, label).FontSize(12).SingleLine().Grow(1).Shrink(1)
						state := x.dev.State
						if state == "unauthorized" {
							state = "allow USB debugging on the phone"
						}
						if x.dev.API != "" {
							state = "Android API " + x.dev.API
						}
						ui.Text(c, state).FontSize(10.5).TextColor(theme.Muted).Shrink(0)
					})
					if row.Clicked() {
						v.picked = x.key()
						v.log, v.logAfter, v.logFor = nil, 0, ""
					}
				}
			})
		}
	})
	ui.Row(c).FillWidth().Gap(6).Children(func() {
		if ok {
			switch {
			case d.avd != "" && d.dev.State == "stopped":
				if widget.Button(c, widget.Secondary, icon.Play, "Start", "").Disabled(v.busy != "").Clicked() {
					v.act(o, "Starting "+d.avd+"…", func() error { return d.host.Client.AndroidStart(d.avd, d.host.Local) })
				}
			case d.dev.State == "device":
				if widget.Button(c, widget.Secondary, icon.AppWindow, "Show screen", "").Disabled(v.busy != "").Clicked() {
					v.showScreen(o, d)
				}
				if c0 := v.caught[d.key()]; c0 != nil {
					if widget.Button(c, widget.Secondary, icon.Traffic, "Stop catching", "").Disabled(v.busy != "").Clicked() {
						v.release(o, d, c0)
					}
				} else if widget.Button(c, widget.Secondary, icon.Traffic, "Catch its traffic", "").Disabled(v.busy != "").Clicked() {
					v.catch(o, d)
				}
				if widget.IconButton(c, icon.Stop, "Stop", false).Clicked() {
					serial := d.dev.Serial
					v.act(o, "", func() error { return d.host.Client.AndroidStop(proto.AndroidDeviceParams{Serial: serial}) })
				}
			}
		}
		ui.Spacer(c)
		if widget.GhostButton(c, "Connect a phone…").Clicked() {
			v.connect.open = !v.connect.open
		}
	})
}

// act runs fn off the main thread, saying what it does.
func (v *View) act(o Options, doing string, fn func() error) {
	v.busy, v.notice = doing, doing
	go func() {
		err := fn()
		o.Update(func() {
			v.busy = ""
			if err != nil {
				v.notice = client.Explain(err)
			} else if v.notice == doing {
				v.notice = ""
			}
		})
	}()
}

// readProject asks the session's host what the session's app is.
func (v *View) readProject(o Options) {
	dir := o.Session.Dir
	if dir == v.projDir || dir == "" {
		return
	}
	v.projDir = dir
	var h *client.Client
	for _, x := range o.Hosts {
		if x.Session {
			h = x.Client
		}
	}
	if h == nil {
		return
	}
	go func() {
		p, err := h.AndroidProject(dir)
		o.Update(func() {
			if err == nil && v.projDir == dir {
				v.project = p
				if v.command == "" {
					v.command = p.Command
				}
			}
		})
	}()
}

func (v *View) runRow(c *ui.Context, o Options) {
	d, ok := v.pickedDevice(o)
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		what := "The session's app"
		if v.project.Kind != "" {
			what = map[string]string{"flutter": "Flutter", "expo": "Expo", "react-native": "React Native", "capacitor": "Capacitor", "nativephp": "NativePHP", "gradle": "Android (Gradle)"}[v.project.Kind] + " app"
		}
		if v.project.Package != "" {
			what += " · " + v.project.Package
		}
		ui.Text(c, what).FontSize(11).TextColor(theme.Muted)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			widget.Input(c, &v.command, "The command that runs it, $ANDROID_SERIAL its device").Grow(1).Font(theme.MonoFont).FontSize(11.5)
			can := ok && d.dev.State == "device" && strings.TrimSpace(v.command) != "" && v.busy == ""
			if widget.PrimaryButton(c, icon.Play, "Run app", "").Disabled(!can).Clicked() {
				v.run(o, d)
			}
		})
	})
}

// run runs the session's app on d in a new tab, bridging ADB when d is
// on another host than the session.
func (v *View) run(o Options, d device) {
	var session Host
	for _, h := range o.Hosts {
		if h.Session {
			session = h
		}
	}
	if session.Client == nil {
		return
	}
	params := proto.AndroidRunParams{Session: o.Session.ID, Serial: d.dev.Serial, Command: strings.TrimSpace(v.command)}
	key := d.key()
	kind := v.project.Kind
	v.act(o, "Starting the app on "+d.label()+"…", func() error {
		if d.host.ID != session.ID {
			b, err := client.BridgeADB(session.Client, d.host.Client)
			if err != nil {
				return err
			}
			params.ADB = b.Addr
			o.Update(func() {
				if old := v.bridges[key]; old != nil {
					old.Close()
				}
				v.bridges[key] = b
			})
			// The phone reaches Metro on its own host: bring it here.
			if (kind == "expo" || kind == "react-native") && o.ForwardPort != nil {
				o.Update(func() { o.ForwardPort(8081) })
			}
		}
		run, err := session.Client.AndroidRun(params)
		if err != nil {
			return err
		}
		o.Update(func() {
			if run.Package != "" {
				v.project.Package = run.Package
			}
			if o.OpenTab != nil {
				o.OpenTab(run.Tab)
			}
		})
		return nil
	})
}

func (v *View) connectView(c *ui.Context, o Options) {
	f := &v.connect
	ui.Column(c).FillWidth().Gap(6).Padding(8).Radius(theme.RadiusControl).Background(theme.Sunken).Children(func() {
		ui.Text(c, "On the phone: Developer options → Wireless debugging. Pair once with a code, then connect.").FontSize(11).TextColor(theme.Muted)
		widget.Input(c, &f.address, "Its address, as Wireless debugging shows: 192.168.1.5:41823").FillWidth()
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			widget.Input(c, &f.pair, "Pairing address (first time)").Grow(1)
			widget.Input(c, &f.code, "Pairing code").Width(120)
		})
		var on Host
		for _, h := range o.Hosts {
			if h.Local || on.Client == nil {
				on = h
			}
		}
		ui.Row(c).FillWidth().Gap(6).Justify(ui.End).Children(func() {
			ui.Text(c, "Through "+on.Name+"'s adb").FontSize(11).TextColor(theme.Muted)
			if widget.PrimaryButton(c, nil, "Connect", "").Disabled(strings.TrimSpace(f.address) == "" || v.busy != "").Clicked() {
				p := proto.AndroidConnectParams{Address: f.address, PairAddress: f.pair, Code: f.code}
				v.act(o, "Connecting…", func() error {
					err := on.Client.AndroidConnect(p)
					if err == nil {
						o.Update(func() { v.connect = connectForm{} })
					}
					return err
				})
			}
		})
	})
}

// showScreen shows d's screen in a scrcpy window here: directly for a
// device of this machine, through the bridge for another host's.
func (v *View) showScreen(o Options, d device) {
	key := d.key()
	if cmd := v.screens[key]; cmd != nil && cmd.ProcessState == nil {
		v.notice = "Its screen is open already."
		return
	}
	v.act(o, "Opening "+d.label()+"'s screen…", func() error {
		sc, err := android.InstallScrcpy(context.Background(), io.Discard)
		if err != nil {
			return err
		}
		p := android.ScreenParams{Serial: d.dev.Serial, Title: d.label()}
		if sdk, ok := android.FindSDK(); ok {
			p.SDKADB = sdk.ADB()
		}
		var cleanup []func()
		if !d.host.Local {
			if o.Local == nil {
				return fmt.Errorf("android: this machine's daemon isn't reachable to show another host's device")
			}
			lc, err := o.Local()
			if err != nil {
				return err
			}
			b, err := client.BridgeADB(lc, d.host.Client)
			if err != nil {
				return err
			}
			cleanup = append(cleanup, b.Close)
			port := 27200 + int(time.Now().UnixNano()%700)
			ln, err := client.ForwardADBPort(d.host.Client, port)
			if err != nil {
				b.Close()
				return err
			}
			cleanup = append(cleanup, func() { ln.Close() })
			p.ADBServer, p.TunnelPort = b.Addr, port
		}
		cmd := sc.Command(p)
		if err := cmd.Start(); err != nil {
			for _, f := range cleanup {
				f()
			}
			return fmt.Errorf("android: scrcpy: %w", err)
		}
		o.Update(func() { v.screens[key] = cmd })
		go func() {
			cmd.Wait()
			for _, f := range cleanup {
				f()
			}
		}()
		return nil
	})
}

// logView is the picked device's log, the app's lines by default.
func (v *View) logView(c *ui.Context, o Options) {
	d, ok := v.pickedDevice(o)
	if !ok || d.dev.State != "device" {
		ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Children(func() {
			ui.Text(c, "The device's log shows here once it runs.").FontSize(12).TextColor(theme.Muted)
		})
		return
	}
	v.readLog(o, d)
	levels := []string{"All", "Info", "Warn", "Error"}
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		ui.Row(c).FillWidth().Shrink(0).Padding(6, 10, 6, 12).Gap(6).Children(func() {
			ui.Text(c, "Logcat").FontSize(11.5).FontWeight(600)
			if v.project.Package != "" {
				widget.Checkbox(c, &v.appOnly, "Only "+v.project.Package)
			}
			ui.Spacer(c)
			if i := widget.Segmented(c, levels, v.level, nil); i >= 0 {
				v.level = i
			}
			if widget.IconButton(c, icon.Trash, "Clear", false).Clicked() {
				v.log = nil
			}
		})
		ui.Row(c).FillWidth().Shrink(0).Padding(0, 10, 6, 12).Children(func() {
			widget.SearchField(c, &v.search, "Search the log", icon.Search)
		})
		lines := v.shownLines()
		ui.List(c.Key("log"), &v.logList, len(lines), func(i int) {
			l := lines[i]
			col := theme.TextSoft
			switch l.Level {
			case "W":
				col = theme.Warning
			case "E", "F", "A":
				col = theme.Error
			case "V", "D":
				col = theme.Muted
			}
			ui.Row(c).FillWidth().Padding(1, 12).Gap(6).AlignItems(ui.Start).Children(func() {
				ui.Text(c, l.Time).Font(theme.MonoFont).FontSize(10.5).TextColor(theme.Muted).Shrink(0)
				ui.Text(c, l.Level).Font(theme.MonoFont).FontSize(10.5).TextColor(col).Shrink(0)
				ui.Text(c, l.Tag).Font(theme.MonoFont).FontSize(10.5).TextColor(theme.Accent).MaxWidth(110).SingleLine().Shrink(0)
				ui.Text(c, l.Message).Font(theme.MonoFont).FontSize(10.5).TextColor(col).Grow(1).Shrink(1).MinWidth(0).Selectable()
			})
		}).Grow(1).FillWidth()
	})
}

func (v *View) shownLines() []proto.AndroidLogLine {
	minLevel := map[int]string{1: "IWEFA", 2: "WEFA", 3: "EFA"}[v.level]
	q := strings.ToLower(v.search)
	var out []proto.AndroidLogLine
	for _, l := range v.log {
		if minLevel != "" && !strings.Contains(minLevel, l.Level) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(l.Tag+" "+l.Message), q) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// readLog asks the device's host for new lines now and then.
func (v *View) readLog(o Options, d device) {
	pkg := ""
	if v.appOnly {
		pkg = v.project.Package
	}
	key := d.key() + "|" + pkg
	if v.logFor != key {
		v.logFor, v.log, v.logAfter = key, nil, 0
	}
	if v.logging {
		return
	}
	v.logging = true
	h, serial, after := d.host.Client, d.dev.Serial, v.logAfter
	go func() {
		l, err := h.AndroidLogcat(proto.AndroidLogcatParams{Serial: serial, Package: pkg, After: after})
		time.Sleep(logEvery)
		o.Update(func() {
			v.logging = false
			if err != nil || v.logFor != key {
				return
			}
			v.log = append(v.log, l.Lines...)
			if len(v.log) > logKept {
				v.log = slices.Clone(v.log[len(v.log)-logKept:])
			}
			v.logAfter = l.Last
		})
	}()
}

// localSetup is Android being set up on this machine, by the app itself:
// the setup's output, and the licenses answered here.
type localSetup struct {
	mu      sync.Mutex
	out     strings.Builder
	answers *io.PipeWriter
	done    bool
	err     string
}

func (s *localSetup) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(p)
	return len(p), nil
}

func (s *localSetup) finished() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

func (s *localSetup) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.out.String()
	if len(t) > 20000 {
		t = t[len(t)-20000:]
	}
	return t
}

// startSetup sets Android up on host h: in a tab of the session on the
// session's host, else here in the app, its output shown in the tool.
func (v *View) startSetup(o Options, h Host) {
	if h.Session {
		v.act(o, "Setting Android up in a tab…", func() error {
			tab, err := h.Client.AndroidSetup(o.Session.ID)
			if err == nil && o.OpenTab != nil {
				o.Update(func() { o.OpenTab(tab.ID) })
			}
			return err
		})
		return
	}
	s := &localSetup{}
	r, w := io.Pipe()
	s.answers = w
	v.setup = s
	go func() {
		err := android.Setup(context.Background(), r, s)
		s.mu.Lock()
		s.done = true
		if err != nil {
			s.err = err.Error()
		}
		s.mu.Unlock()
		o.Update(func() {})
	}()
	// The output shows as it comes.
	go func() {
		for !s.finished() {
			time.Sleep(time.Second)
			o.Update(func() {})
		}
	}()
}

func (v *View) setupView(c *ui.Context, o Options) {
	s := v.setup
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(10, 12).Gap(8).Children(func() {
		ui.Text(c, "Setting Android up on this machine").FontSize(12.5).FontWeight(600)
		widget.LogView(c, s.text(), 360)
		s.mu.Lock()
		done, failed := s.done, s.err
		s.mu.Unlock()
		if failed != "" {
			ui.Text(c, failed).FontSize(12).TextColor(theme.Error)
		}
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if !done && widget.Button(c, widget.Secondary, icon.Check, "Accept the licenses shown", "").Clicked() {
				go io.WriteString(s.answers, strings.Repeat("y\n", 12))
			}
			if done && widget.PrimaryButton(c, icon.Check, "Close", "").Clicked() {
				v.setup = nil
			}
		})
	})
}

// catch sends device d's HTTP through the session's host's interceptor:
// a source there, its port reached from d's host (forwarded here when d
// is on this machine and the session on a server), and d's proxy and
// trust set by its host.
func (v *View) catch(o Options, d device) {
	var session Host
	for _, h := range o.Hosts {
		if h.Session {
			session = h
		}
	}
	if session.Client == nil {
		return
	}
	key, serial := d.key(), d.dev.Serial
	v.act(o, "Catching "+d.label()+"'s traffic…", func() error {
		t, err := session.Client.TrafficAttach(proto.TrafficAttachParams{Kind: proto.TargetAndroid, Session: o.Session.ID, Device: serial, Label: d.label()})
		if err != nil {
			return err
		}
		ca, err := session.Client.TrafficCA()
		if err != nil {
			return err
		}
		c := &caught{source: t.Source, host: d.host, serial: serial}
		if d.host.ID == session.ID {
			u, err := url.Parse(t.Proxy)
			if err != nil {
				return err
			}
			c.port, _ = strconv.Atoi(u.Port())
		} else {
			ln, err := session.Client.ForwardTrafficSource(t.Source)
			if err != nil {
				return err
			}
			c.ln, c.port = ln, ln.Addr().(*net.TCPAddr).Port
		}
		res, err := d.host.Client.AndroidProxy(proto.AndroidProxyParams{Serial: serial, Port: c.port, CA: ca.PEM})
		if err != nil {
			if c.ln != nil {
				c.ln.Close()
			}
			session.Client.TrafficDetach(t.Source)
			return err
		}
		o.Update(func() {
			v.caught[key] = c
			v.notice = "Its traffic shows in Traffic. " + res.Note
		})
		return nil
	})
}

func (v *View) release(o Options, d device, c *caught) {
	var session Host
	for _, h := range o.Hosts {
		if h.Session {
			session = h
		}
	}
	key := d.key()
	delete(v.caught, key)
	v.act(o, "", func() error {
		c.host.Client.AndroidProxy(proto.AndroidProxyParams{Serial: c.serial, Port: c.port, Off: true})
		if c.ln != nil {
			c.ln.Close()
		}
		if session.Client != nil {
			return session.Client.TrafficDetach(c.source)
		}
		return nil
	})
}
