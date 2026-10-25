package daemon

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// fakeSDK is an SDK whose adb and emulator are scripts answering as the
// real ones would for one emulator and one phone.
func fakeSDK(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, script string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("platform-tools/adb", `
case "$*" in
  "devices -l") printf 'List of devices attached\nemulator-5554 device product:x model:sdk_gphone64 device:emu\nR58 device usb:1 model:Pixel_7\n\n' ;;
  *"emu avd name"*) printf 'enowX_Phone\nOK\n' ;;
  *"ro.product.cpu.abi"*) echo x86_64 ;;
  *"ro.build.version.sdk"*) echo 35 ;;
  *"pidof com.example.app"*) echo 4242 ;;
  *"forward tcp:0 localabstract:scrcpy_"*) echo "$FAKE_SCRCPY_PORT" ;;
  *logcat*) printf '10-25 14:13:45.123  4242  4250 I flutter: hello from the app\n10-25 14:13:45.200  1000  1001 W System: other\n'; sleep 30 ;;
  *) echo "$*" >> "$(dirname "$0")/calls" ;;
esac
`)
	write("emulator/emulator", `
case "$*" in
  "-list-avds") echo enowX_Phone ;;
  "-accel-check") echo "accel:"; echo 0; echo "KVM is installed and usable." ;;
  *) sleep 30 ;;
esac
`)
	return root
}

// A host's Android: its SDK, emulator and devices; a device's log for an
// app; and a session's app run in a tab with the device in its
// environment.
func TestAndroidOnAHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("ANDROID_HOME", fakeSDK(t))
	srv := startServer(t)
	c := connect(t, srv, nil)
	st, err := c.AndroidStatus()
	if err != nil {
		t.Fatal(err)
	}
	if st.Problem != "" || !st.Accel || len(st.AVDs) != 1 || st.AVDs[0] != "enowX_Phone" || len(st.Devices) != 2 {
		t.Fatalf("status %+v", st)
	}
	if d := st.Devices[0]; d.Name != "enowX_Phone" || d.Kind != "emulator" || d.API != "35" || d.ABI != "x86_64" {
		t.Fatalf("emulator %+v", d)
	}
	if d := st.Devices[1]; d.Name != "Pixel 7" || d.Kind != "physical" {
		t.Fatalf("phone %+v", d)
	}

	var log proto.AndroidLog
	waitFor(t, "the app's log", func() bool {
		log, _ = c.AndroidLogcat(proto.AndroidLogcatParams{Serial: "emulator-5554", Package: "com.example.app"})
		return len(log.Lines) == 1
	})
	if l := log.Lines[0]; l.Tag != "flutter" || l.Message != "hello from the app" || l.Level != "I" {
		t.Fatalf("log %+v", log)
	}

	project := t.TempDir()
	os.WriteFile(filepath.Join(project, "pubspec.yaml"), []byte("name: app\nflutter:\n"), 0o644)
	if _, err := c.AddProject(project); err != nil {
		t.Fatal(err)
	}
	sess, err := c.CreateSession(proto.SessionCreateParams{Project: project, Agent: "shell", BranchMode: proto.BranchNone})
	if err != nil {
		t.Fatal(err)
	}
	run, err := c.AndroidRun(proto.AndroidRunParams{Session: sess.ID, Serial: "emulator-5554", Command: `echo "on $ANDROID_SERIAL via $ADB_SERVER_SOCKET"`, ADB: "127.0.0.1:41001"})
	if err != nil {
		t.Fatal(err)
	}
	tab, _ := srv.sessions.Get(run.Tab)
	waitFor(t, "the run tab", func() bool {
		return strings.Contains(tab.Terminal().Text(), "on emulator-5554 via tcp:127.0.0.1:41001")
	})
	if p, _ := c.AndroidProject(project); p.Kind != "flutter" || !strings.Contains(p.Command, "flutter run") {
		t.Fatalf("project %+v", p)
	}
}

var bigAnswer = []byte(strings.Repeat("logcat line\n", 350_000))

// The bridge carries a session's host's ADB connections to the device's
// host's ADB server, both ways.
func TestADBBridge(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("ANDROID_HOME", fakeSDK(t))
	adb, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer adb.Close()
	go func() {
		for {
			c, err := adb.Accept()
			if err != nil {
				return
			}
			go func() {
				// As adb's server does: answer, then close at once; a
				// large answer is still on its way as the close comes.
				buf := make([]byte, 64)
				n, _ := c.Read(buf)
				c.Write(append(append([]byte("OKAY"), buf[:n]...), bigAnswer...))
				c.Close()
			}()
		}
	}()
	old := adbPort
	adbPort = adb.Addr().String()
	defer func() { adbPort = old }()

	sessionHost, deviceHost := startServer(t), startServer(t)
	sc, dc := connect(t, sessionHost, nil), connect(t, deviceHost, nil)
	b, err := client.BridgeADB(sc, dc)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	for i := range 2 { // a tool connects again and again
		c, err := net.Dial("tcp", b.Addr)
		if err != nil {
			t.Fatal(err)
		}
		msg := fmt.Sprintf("000chost:devices #%d", i)
		c.Write([]byte(msg))
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		got, err := io.ReadAll(c)
		if err != nil || string(got) != "OKAY"+msg+string(bigAnswer) {
			t.Fatalf("through the bridge: %d bytes, %v", len(got), err)
		}
		c.Close()
	}
}

// A device caught by Traffic: adb reverse brings its proxy port home,
// its proxy setting points there, and an emulator gets the CA among its
// user certificates.
func TestAndroidProxy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	root := fakeSDK(t)
	t.Setenv("ANDROID_HOME", root)
	srv := startServer(t)
	c := connect(t, srv, nil)
	ca, err := c.TrafficCA()
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.AndroidProxy(proto.AndroidProxyParams{Serial: "emulator-5554", Port: 41234, CA: ca.PEM})
	if err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(filepath.Join(root, "platform-tools", "calls"))
	for _, want := range []string{"-s emulator-5554 reverse tcp:41234 tcp:41234", "settings put global http_proxy 127.0.0.1:41234", "-s emulator-5554 root", "/data/misc/user/0/cacerts-added/"} {
		if !strings.Contains(string(calls), want) {
			t.Errorf("no %q in adb's calls:\n%s", want, calls)
		}
	}
	if !strings.Contains(res.Note, "user certificates") {
		t.Errorf("note %q", res.Note)
	}
}
