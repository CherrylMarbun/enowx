package android

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/internal/quietcmd"
	"github.com/enowx/enowx/proto"
)

// toolTimeout bounds a quick call of adb or the emulator.
const toolTimeout = 15 * time.Second

// Status is what this host has for Android, and why what is missing is.
func Status() proto.AndroidStatus {
	s, found := FindSDK()
	st := proto.AndroidStatus{Java: JavaVersion()}
	if !found {
		st.Problem = "No Android SDK here: Set up Android brings it, with an emulator and a virtual phone."
		return st
	}
	st.SDK = s.Root
	st.ADB, st.Emulator = fileExists(s.ADB()), fileExists(s.Emulator())
	if st.Emulator {
		st.Accel, st.AccelWhy = s.Accel()
		st.AVDs = s.AVDs()
	}
	if st.ADB {
		st.Devices = s.Devices()
	}
	return st
}

// Accel reports whether the emulator is accelerated here, and why not.
func (s SDK) Accel() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	out, err := s.command(ctx, s.Emulator(), "-accel-check").CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err == nil {
		return true, ""
	}
	why := lastLine(text)
	switch runtime.GOOS {
	case "linux":
		if !fileExists("/dev/kvm") {
			why = "No /dev/kvm: this machine (or its VM) doesn't offer KVM to Linux. A server needs nested virtualization or a dedicated host; the emulator runs here only very slowly."
		} else {
			why += " (is your user in the kvm group?)"
		}
	case "windows":
		why += " Turn on Windows Hypervisor Platform in Windows Features."
	}
	return false, why
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// AVDs lists the virtual devices the emulator can start.
func (s SDK) AVDs() []string {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	out, err := s.command(ctx, s.Emulator(), "-list-avds").Output()
	if err != nil {
		return nil
	}
	var avds []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.Contains(line, " ") {
			avds = append(avds, line)
		}
	}
	return avds
}

// Devices lists the devices adb sees, emulators named by their AVD.
func (s SDK) Devices() []proto.AndroidDevice {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	out, err := s.command(ctx, s.ADB(), "devices", "-l").Output()
	if err != nil {
		return nil
	}
	devs := parseDevices(string(out))
	for i := range devs {
		d := &devs[i]
		if d.State != "device" {
			continue
		}
		if d.Kind == "emulator" {
			if name, err := s.adb(d.Serial, "emu", "avd", "name"); err == nil {
				if n := strings.TrimSpace(strings.Split(name, "\n")[0]); n != "" && n != "OK" {
					d.Name = n
				}
			}
		}
		d.ABI, _ = s.shell(d.Serial, "getprop ro.product.cpu.abi")
		d.API, _ = s.shell(d.Serial, "getprop ro.build.version.sdk")
		if c, _ := s.shell(d.Serial, "getprop ro.build.characteristics"); strings.Contains(c, "tv") {
			d.TV = true
		}
	}
	return devs
}

// parseDevices reads `adb devices -l`.
func parseDevices(out string) []proto.AndroidDevice {
	var devs []proto.AndroidDevice
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] == "List" || strings.HasPrefix(f[0], "*") {
			continue
		}
		d := proto.AndroidDevice{Serial: f[0], State: f[1], Kind: "physical"}
		if strings.HasPrefix(d.Serial, "emulator-") {
			d.Kind = "emulator"
		}
		for _, kv := range f[2:] {
			if m, ok := strings.CutPrefix(kv, "model:"); ok {
				d.Name = strings.ReplaceAll(m, "_", " ")
			}
		}
		if d.Name == "" {
			d.Name = d.Serial
		}
		devs = append(devs, d)
	}
	return devs
}

// adb runs adb for device serial ("" for none) and returns its output.
func (s SDK) adb(serial string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	if serial != "" {
		args = append([]string{"-s", serial}, args...)
	}
	out, err := s.command(ctx, s.ADB(), args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("adb %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// ADBRun runs adb with args against device serial, as the daemon's
// actions need.
func (s SDK) ADBRun(serial string, args ...string) (string, error) { return s.adb(serial, args...) }

func (s SDK) shell(serial, cmd string) (string, error) {
	out, err := s.adb(serial, "shell", cmd)
	return strings.TrimSpace(out), err
}

// Start starts virtual device avd in the background: with its own window,
// or headless. An emulator that gives up at once (no room for its data,
// no acceleration it insists on) is an error with its reason; else it
// returns while the emulator boots, which Wait waits for.
func (s SDK) Start(avd string, window bool) (*exec.Cmd, error) {
	if !slices.Contains(s.AVDs(), avd) {
		return nil, fmt.Errorf("android: no virtual device %q here", avd)
	}
	args := []string{"-avd", avd, "-no-snapshot-save", "-no-boot-anim"}
	if !window {
		args = append(args, "-no-window", "-no-audio")
	}
	cmd := quietcmd.Command(s.Emulator(), args...)
	cmd.Env = s.toolEnv()
	out := &tailBuffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("android: start %s: %w", avd, err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		return nil, fmt.Errorf("android: the emulator stopped: %s", out.reason(err))
	case <-time.After(startCheck):
	}
	return cmd, nil
}

// startCheck is how long an emulator has to give up at once.
const startCheck = 6 * time.Second

// tailBuffer keeps the end of a program's output. It is safe for use by
// several goroutines.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > 16<<10 {
		b.buf = b.buf[len(b.buf)-16<<10:]
	}
	return len(p), nil
}

// reason is the emulator's FATAL or ERROR line, else its last.
func (b *tailBuffer) reason(err error) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(b.buf)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "FATAL") || strings.HasPrefix(l, "ERROR") {
			return strings.TrimSpace(strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(l, "FATAL"), "ERROR"), " |"))
		}
	}
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		return lines[len(lines)-1]
	}
	return fmt.Sprint(err)
}

// Wait waits until device serial has booted, or ctx ends.
func (s SDK) Wait(ctx context.Context, serial string) error {
	for {
		if out, err := s.shell(serial, "getprop sys.boot_completed"); err == nil && strings.TrimSpace(out) == "1" {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("android: the device didn't boot in time")
		case <-time.After(2 * time.Second):
		}
	}
}

// Stop stops device serial: an emulator ends, a phone is disconnected
// when it is connected over the network.
func (s SDK) Stop(serial string) error {
	if strings.HasPrefix(serial, "emulator-") {
		_, err := s.adb(serial, "emu", "kill")
		return err
	}
	if strings.Contains(serial, ":") {
		_, err := s.adb("", "disconnect", serial)
		return err
	}
	return errors.New("android: a phone on USB stays: unplug it")
}

// Connect connects a phone over wireless debugging, pairing it first with
// its code when given.
func (s SDK) Connect(address, pairAddress, code string) error {
	if code != "" {
		if pairAddress == "" {
			pairAddress = address
		}
		out, err := s.adb("", "pair", pairAddress, code)
		if err != nil || !strings.Contains(strings.ToLower(out), "success") {
			return fmt.Errorf("android: pairing failed: %s", strings.TrimSpace(out))
		}
	}
	out, err := s.adb("", "connect", address)
	if err != nil || strings.Contains(out, "failed") || strings.Contains(out, "cannot") {
		return fmt.Errorf("android: %s", strings.TrimSpace(out))
	}
	return nil
}

// StartServer starts adb's server, so tools across the bridge find it.
func (s SDK) StartServer() error {
	_, err := s.adb("", "start-server")
	return err
}
