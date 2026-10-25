package android

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/enowx/enowx/internal/quietcmd"
)

// scrcpy (Apache-2.0) shows a device's screen in a window of its own on
// this machine and sends touch, keys and the clipboard back. enowX uses
// the one installed, or downloads this release for this platform once,
// checked against its checksum.
const scrcpyVersion = "v5.0.1"

// scrcpyArchives are the release's archives by platform, with their
// SHA-256 as the release lists them.
var scrcpyArchives = map[string]struct{ name, sha256 string }{
	"linux/amd64":   {"scrcpy-linux-x86_64-v5.0.1.tar.gz", "9f969d30cc574816077edecda65719c1c70b0aee1df1ba5fa00779ac07b8fd6e"},
	"darwin/arm64":  {"scrcpy-macos-aarch64-v5.0.1.tar.gz", "33611e51977a8289e2e124b220f727895181ef2eb9060e0987a4dc2db9cf0d6b"},
	"darwin/amd64":  {"scrcpy-macos-x86_64-v5.0.1.tar.gz", "31a5467a9e907f162b093267cbbab1276b15c6e709c2ea8159566a8d165beb50"},
	"windows/amd64": {"scrcpy-win64-v5.0.1.zip", "b12a2c4ee8be317422451fc7dcf8ee20a71b5ea7ef9ad73ddd825a25316227a5"},
	"windows/386":   {"scrcpy-win32-v5.0.1.zip", "0cb65ebf1bce892fa4d1502d699af0c7a1a7f24d7febcb9ef72894e3f23fc2ad"},
	"windows/arm64": {"scrcpy-winarm64-v5.0.1.zip", "16c3fc2068df64946f670e3504b4752073ebae47a416a45b46e79a5ab19c689b"},
}

// Scrcpy is scrcpy on this machine, and the adb it comes with.
type Scrcpy struct {
	Path string
	ADB  string // "" to use the adb on PATH or the SDK's
}

// scrcpyDir is where enowX keeps its scrcpy.
func scrcpyDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".enowx-app", "tools", "scrcpy-"+scrcpyVersion)
}

// FindScrcpy finds scrcpy: enowX's, else one installed.
func FindScrcpy() (Scrcpy, bool) {
	dir := scrcpyDir()
	if fileExists(filepath.Join(dir, exe("scrcpy"))) {
		s := Scrcpy{Path: filepath.Join(dir, exe("scrcpy"))}
		if fileExists(filepath.Join(dir, exe("adb"))) {
			s.ADB = filepath.Join(dir, exe("adb"))
		}
		return s, true
	}
	if p, err := exec.LookPath("scrcpy"); err == nil {
		return Scrcpy{Path: p}, true
	}
	return Scrcpy{}, false
}

// InstallScrcpy downloads scrcpy for this platform, once.
func InstallScrcpy(ctx context.Context, out io.Writer) (Scrcpy, error) {
	if s, ok := FindScrcpy(); ok {
		return s, nil
	}
	a, ok := scrcpyArchives[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return Scrcpy{}, fmt.Errorf("android: scrcpy has no release for %s/%s: install it yourself", runtime.GOOS, runtime.GOARCH)
	}
	url := "https://github.com/Genymobile/scrcpy/releases/download/" + scrcpyVersion + "/" + a.name
	tmp := filepath.Join(os.TempDir(), a.name)
	if err := download(ctx, url, tmp, out); err != nil {
		return Scrcpy{}, err
	}
	defer os.Remove(tmp)
	if err := checkSHA256(tmp, a.sha256); err != nil {
		return Scrcpy{}, err
	}
	dir := scrcpyDir()
	os.RemoveAll(dir)
	var err error
	if filepath.Ext(a.name) == ".zip" {
		err = unzip(tmp, dir, "*")
	} else {
		err = untar(tmp, dir)
	}
	if err != nil {
		return Scrcpy{}, err
	}
	s, ok := FindScrcpy()
	if !ok {
		return Scrcpy{}, fmt.Errorf("android: scrcpy isn't where its archive should have put it")
	}
	return s, nil
}

func checkSHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("android: %s is damaged: its SHA-256 is %s, not %s", filepath.Base(path), got, want)
	}
	return nil
}

// ScreenParams say which device's screen to show and how to reach it.
type ScreenParams struct {
	Serial string
	Title  string
	// ADBServer is a bridged ADB server's address (host:port) when the
	// device is on another host; TunnelPort is then the port scrcpy's
	// video comes through, forwarded here.
	ADBServer  string
	TunnelPort int
	// SDKADB is the SDK's adb, used when scrcpy brings none.
	SDKADB string
}

// Command is scrcpy showing a device's screen.
func (s Scrcpy) Command(p ScreenParams) *exec.Cmd {
	args := []string{"-s", p.Serial, "--window-title", p.Title, "--stay-awake", "--no-audio"}
	env := os.Environ()
	if p.ADBServer != "" {
		// adb forward, not reverse: the device's host's adb server listens,
		// and the port comes here through the bridge.
		args = append(args, "--force-adb-forward", "--tunnel-host=127.0.0.1", "--tunnel-port="+strconv.Itoa(p.TunnelPort))
		env = append(env, "ADB_SERVER_SOCKET=tcp:"+p.ADBServer)
	}
	switch {
	case s.ADB != "":
		env = append(env, "ADB="+s.ADB)
	case p.SDKADB != "":
		env = append(env, "ADB="+p.SDKADB)
	}
	cmd := quietcmd.Command(s.Path, args...)
	cmd.Env = env
	cmd.Dir = filepath.Dir(s.Path) // where scrcpy-server is
	return cmd
}
