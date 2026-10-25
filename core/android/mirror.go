package android

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/internal/quietcmd"
)

// A device's screen shows in the app through scrcpy's server (Apache-2.0),
// which runs on the device: it sends the screen as H.264 on one socket and
// takes touches and keys on another. The device's host starts it and
// connects both through adb; the video goes on to the app as it is.

// scrcpyServer is the server's release, with its SHA-256 as the release
// lists it. The version is also what the server checks it is told.
const (
	scrcpyServerVersion = "5.0.1"
	scrcpyServerSHA256  = "764eb6f79811d5211fe9df341120882ba9994c7a61b897d7bf3fb662e53bc536"
	serverOnDevice      = "/data/local/tmp/enowx-scrcpy-server.jar"
)

// ServerJar is scrcpy's server on this host, downloaded once.
func ServerJar(ctx context.Context) (string, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".enowx-app", "tools", "scrcpy-server-v"+scrcpyServerVersion)
	if checkSHA256(path, scrcpyServerSHA256) == nil {
		return path, nil
	}
	url := "https://github.com/Genymobile/scrcpy/releases/download/v" + scrcpyServerVersion + "/scrcpy-server-v" + scrcpyServerVersion
	tmp := path + ".part"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := download(ctx, url, tmp, io.Discard); err != nil {
		return "", err
	}
	if err := checkSHA256(tmp, scrcpyServerSHA256); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return path, os.Rename(tmp, path)
}

// MirrorOptions are how a screen is sent.
type MirrorOptions struct {
	// MaxSize caps the video's longer side, in pixels; 0 is the device's.
	MaxSize int
	BitRate int // bits a second; 0 for 4 Mbps
	MaxFPS  int // 0 for 60
}

// Mirror is a device's screen being sent: Video is scrcpy's video stream
// (the device's name in 64 bytes, the codec in 4, then 12-byte headers:
// a session's, its first bit set, with the screen's width and height at
// 4 and 8; else a packet's, bit 62 for config and 61 for a key frame, the
// rest its time, then its size and H.264 in Annex B); the methods send
// input.
type Mirror struct {
	Video   net.Conn
	sdk     SDK
	serial  string
	port    int
	cmd     *exec.Cmd
	control net.Conn
	log     *lockedBuffer

	mu     sync.Mutex // guards control's writes
	closed sync.Once
}

// Mirror starts scrcpy's server on device serial and connects to it.
func (s SDK) Mirror(ctx context.Context, serial string, opt MirrorOptions) (*Mirror, error) {
	jar, err := ServerJar(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.adb(serial, "push", jar, serverOnDevice); err != nil {
		return nil, err
	}
	var id [4]byte
	rand.Read(id[:])
	id[0] &= 0x7f // a positive 31-bit number, as the server parses it
	scid := hex.EncodeToString(id[:])
	out, err := s.adb(serial, "forward", "tcp:0", "localabstract:scrcpy_"+scid)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return nil, fmt.Errorf("android: adb forward said %q", strings.TrimSpace(out))
	}
	m := &Mirror{sdk: s, serial: serial, port: port, log: &lockedBuffer{}}
	if opt.BitRate == 0 {
		opt.BitRate = 4_000_000
	}
	if opt.MaxFPS == 0 {
		opt.MaxFPS = 60
	}
	args := []string{"-s", serial, "shell", "CLASSPATH=" + serverOnDevice, "app_process", "/", "com.genymobile.scrcpy.Server", scrcpyServerVersion,
		"scid=" + scid, "log_level=info", "audio=false", "control=true", "tunnel_forward=true", "video_codec=h264",
		"max_size=" + strconv.Itoa(opt.MaxSize), "max_fps=" + strconv.Itoa(opt.MaxFPS), "video_bit_rate=" + strconv.Itoa(opt.BitRate),
		"send_device_meta=true", "send_dummy_byte=true", "send_frame_meta=true", "send_codec_meta=true",
		"cleanup=true", "clipboard_autosync=false", "power_off_on_close=false"}
	m.cmd = quietcmd.Command(s.ADB(), args...)
	m.cmd.Env = s.toolEnv()
	m.cmd.Stdout, m.cmd.Stderr = m.log, m.log
	if err := m.cmd.Start(); err != nil {
		m.Close()
		return nil, err
	}
	go m.cmd.Wait()
	// The forward accepts before the server listens, then drops the
	// connection: the dummy byte says it got there.
	deadline := time.Now().Add(8 * time.Second)
	for {
		c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), time.Second)
		if err == nil {
			c.SetReadDeadline(time.Now().Add(2 * time.Second))
			var b [1]byte
			if _, err = io.ReadFull(c, b[:]); err == nil {
				c.SetReadDeadline(time.Time{})
				m.Video = c
				break
			}
			c.Close()
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			why := strings.TrimSpace(m.log.String())
			m.Close()
			if why == "" {
				why = "it never answered"
			}
			return nil, fmt.Errorf("android: the screen didn't start on %s: %s", serial, lastLines(why, 3))
		}
		time.Sleep(100 * time.Millisecond)
	}
	if m.control, err = net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port)); err != nil {
		m.Close()
		return nil, err
	}
	// What the device sends back (its clipboard) isn't used.
	go io.Copy(io.Discard, m.control)
	return m, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " ")
}

// Close stops the server and its forward.
func (m *Mirror) Close() error {
	m.closed.Do(func() {
		if m.Video != nil {
			m.Video.Close()
		}
		if m.control != nil {
			m.control.Close()
		}
		if m.cmd != nil && m.cmd.Process != nil {
			m.cmd.Process.Kill()
		}
		m.sdk.adb(m.serial, "forward", "--remove", "tcp:"+strconv.Itoa(m.port))
	})
	return nil
}

// Actions of a touch or a key.
const (
	ActionDown = 0
	ActionUp   = 1
	ActionMove = 2
)

// Android's key codes the screen's buttons send.
const (
	KeyHome       = 3
	KeyBack       = 4
	KeyVolumeUp   = 24
	KeyVolumeDown = 25
	KeyPower      = 26
	KeyEnter      = 66
	KeyDelete     = 67
	KeyAppSwitch  = 187
)

// Control messages' types.
const (
	msgKey   = 0
	msgText  = 1
	msgTouch = 2
)

// pointerFinger is the pointer a touch is: a finger, not a mouse, so apps
// see a touch screen.
const pointerFinger = ^uint64(1) // POINTER_ID_GENERIC_FINGER, -2

func (m *Mirror) send(b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.control == nil {
		return errors.New("android: the screen is closed")
	}
	m.control.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err := m.control.Write(b)
	return err
}

// Touch touches the screen at x, y of a video width by height (the size
// the video has now: the server drops touches of another size).
func (m *Mirror) Touch(action, x, y, width, height int) error {
	b := make([]byte, 32)
	b[0], b[1] = msgTouch, byte(action)
	binary.BigEndian.PutUint64(b[2:], pointerFinger)
	binary.BigEndian.PutUint32(b[10:], uint32(int32(x)))
	binary.BigEndian.PutUint32(b[14:], uint32(int32(y)))
	binary.BigEndian.PutUint16(b[18:], uint16(width))
	binary.BigEndian.PutUint16(b[20:], uint16(height))
	pressure := uint16(0xffff)
	if action == ActionUp {
		pressure = 0
	}
	binary.BigEndian.PutUint16(b[22:], pressure)
	// action button and buttons: none, for a finger.
	return m.send(b)
}

// Key presses or releases a key.
func (m *Mirror) Key(action, code int) error {
	b := make([]byte, 14)
	b[0], b[1] = msgKey, byte(action)
	binary.BigEndian.PutUint32(b[2:], uint32(code))
	return m.send(b)
}

// Press presses a key and lets it go.
func (m *Mirror) Press(code int) error {
	if err := m.Key(ActionDown, code); err != nil {
		return err
	}
	return m.Key(ActionUp, code)
}

// Text types text, as a keyboard would.
func (m *Mirror) Text(text string) error {
	if len(text) > 300 {
		text = text[:300]
	}
	b := make([]byte, 5+len(text))
	b[0] = msgText
	binary.BigEndian.PutUint32(b[1:], uint32(len(text)))
	copy(b[5:], text)
	return m.send(b)
}

// lockedBuffer is the server's output, written by the process's copier
// while a failure reads it.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
