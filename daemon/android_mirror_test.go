package daemon

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/core/android"
	"github.com/enowx/enowx/proto"
)

// A device's screen: the daemon starts scrcpy's server on it through adb,
// the video comes to the client as a tunnel, a touch goes back to the
// server's control socket, and closing the tunnel stops it all.
func TestAndroidScreen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	// The server's jar: the one this machine has, else downloaded once.
	jar, err := android.ServerJar(t.Context())
	if err != nil {
		t.Skip("no scrcpy server to push:", err)
	}
	sdk := fakeSDK(t)
	t.Setenv("ANDROID_HOME", sdk)
	home := t.TempDir()
	t.Setenv("HOME", home)
	tools := filepath.Join(home, ".enowx-app", "tools")
	os.MkdirAll(tools, 0o755)
	if b, err := os.ReadFile(jar); err == nil {
		os.WriteFile(filepath.Join(tools, filepath.Base(jar)), b, 0o644)
	}

	// The device: the first connection is the video, after a dummy byte;
	// the second the control.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	t.Setenv("FAKE_SCRCPY_PORT", strconv.Itoa(ln.Addr().(*net.TCPAddr).Port))
	control := make(chan []byte, 1)
	go func() {
		video, err := ln.Accept()
		if err != nil {
			return
		}
		defer video.Close()
		meta := make([]byte, 80)
		copy(meta, "Pixel 7")
		copy(meta[64:], "h264")
		meta[68] = 0x80 // a session: the screen's size
		binary.BigEndian.PutUint32(meta[72:], 540)
		binary.BigEndian.PutUint32(meta[76:], 1200)
		video.Write(append([]byte{0}, meta...))
		packet := make([]byte, 12)
		binary.BigEndian.PutUint64(packet, 1<<62)
		binary.BigEndian.PutUint32(packet[8:], 4)
		video.Write(append(packet, 0, 0, 1, 0x67))
		ctl, err := ln.Accept()
		if err != nil {
			return
		}
		defer ctl.Close()
		b := make([]byte, 32)
		io.ReadFull(ctl, b)
		control <- b
		io.Copy(io.Discard, video) // until the screen closes
	}()

	srv := startServer(t)
	c := connect(t, srv, nil)
	tun, err := c.AndroidMirror(proto.AndroidMirrorParams{Serial: "R58", MaxSize: 1280})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 80+12+4)
	if _, err := io.ReadFull(tun, got); err != nil {
		t.Fatal(err)
	}
	if string(got[64:68]) != "h264" || binary.BigEndian.Uint32(got[76:80]) != 1200 || !bytes.Equal(got[92:], []byte{0, 0, 1, 0x67}) {
		t.Fatalf("video % x", got)
	}
	if err := c.AndroidMirrorInput(proto.AndroidMirrorInput{Serial: "R58", Kind: "touch", Action: 0, X: 100, Y: 200, Width: 540, Height: 1200}); err != nil {
		t.Fatal(err)
	}
	select {
	case b := <-control:
		if b[0] != 2 || b[1] != 0 || binary.BigEndian.Uint32(b[10:]) != 100 || binary.BigEndian.Uint32(b[14:]) != 200 || binary.BigEndian.Uint16(b[18:]) != 540 {
			t.Fatalf("touch % x", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no touch reached the device")
	}
	waitFor(t, "the server started on the device", func() bool {
		calls, _ := os.ReadFile(filepath.Join(sdk, "platform-tools", "calls"))
		return strings.Contains(string(calls), "push") && strings.Contains(string(calls), "com.genymobile.scrcpy.Server 5.0.1 scid=")
	})
	tun.Close()
	waitFor(t, "the screen stopped", func() bool {
		calls, _ := os.ReadFile(filepath.Join(sdk, "platform-tools", "calls"))
		return strings.Contains(string(calls), "forward --remove")
	})
	if err := c.AndroidMirrorInput(proto.AndroidMirrorInput{Serial: "R58", Kind: "key", Key: android.KeyHome}); err == nil {
		t.Fatal("input after the screen closed")
	}
}
