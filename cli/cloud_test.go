package cli_test

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
)

// A server joins the team cloud from its command line: the address, a
// login approved from the user's Mac, then teams made and an invite given.
func TestCloudCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the daemon's socket the way the others here do")
	}
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "https://cloud.example.com"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	acc, _ := db.GitHubAccount(ctx, 1, "ada", "Ada", "", "")
	k, _ := seal.NewDeviceKey()
	_, macToken, _ := db.AddDevice(ctx, acc.ID, "Ada's Mac", api.DeviceApp, k.Public())
	mac, _ := cloudclient.New(cloud.URL, macToken)

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "enowxd")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/enowx/enowx/cmd/enowxd").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	home, err := os.MkdirTemp("", "enx")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	env := append(os.Environ(), "ENOWX_HOME="+home, "ENOWX_SESSION=")
	serve := exec.Command(bin, "serve")
	serve.Env = env
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stop := exec.Command(bin, "stop")
		stop.Env = env
		_ = stop.Run()
		_ = serve.Wait()
	}()
	run := func(args ...string) string {
		t.Helper()
		var out []byte
		var err error
		for range 50 {
			cmd := exec.Command(bin, args...)
			cmd.Env = env
			if out, err = cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "isn't running") {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("enowxd %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}

	if out := run("cloud"); !strings.Contains(out, "The cloud is off") {
		t.Fatalf("cloud at first: %s", out)
	}
	run("cloud", "url", cloud.URL)
	login := exec.Command(bin, "cloud", "login", "--name", "vps")
	login.Env = env
	stdout, _ := login.StdoutPipe()
	if err := login.Start(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(stdout)
	code := ""
	var said strings.Builder
	for code == "" && lines.Scan() {
		said.WriteString(lines.Text() + "\n")
		if m := regexp.MustCompile(`^Code: ([A-Z]{4}-[A-Z]{4})$`).FindStringSubmatch(lines.Text()); m != nil {
			code = m[1]
		}
	}
	if code == "" {
		t.Fatalf("no code shown: %s", said.String())
	}
	if err := mac.ApproveDevice(ctx, code); err != nil {
		t.Fatal(err)
	}
	for lines.Scan() {
		said.WriteString(lines.Text() + "\n")
	}
	if err := login.Wait(); err != nil || !strings.Contains(said.String(), `Signed in to `+cloud.URL+` as Ada (@ada), this host being "vps" (host)`) {
		t.Fatalf("login: %v\n%s", err, said.String())
	}
	if out := run("cloud", "whoami"); !strings.Contains(out, "as Ada") {
		t.Fatalf("whoami: %s", out)
	}
	if out := run("cloud", "devices"); !strings.Contains(out, "vps (this host)") || !strings.Contains(out, "Ada's Mac") {
		t.Fatalf("devices: %s", out)
	}
	if out := run("team", "create", "enowX", "Labs"); !strings.Contains(out, "Made team enowX Labs") {
		t.Fatalf("create: %s", out)
	}
	if out := run("team", "invite", "enowx labs", "--role", "viewer"); !strings.Contains(out, "as viewer") || !strings.Contains(out, "https://cloud.example.com/invite#enxi_") {
		t.Fatalf("invite: %s", out)
	}
	if out := run("team", "members", "enowX Labs"); !strings.Contains(out, "Ada") || !strings.Contains(out, "owner") {
		t.Fatalf("members: %s", out)
	}
	if out := run("cloud", "logout"); !strings.Contains(out, "Not signed in") {
		t.Fatalf("logout: %s", out)
	}
	cmd := exec.Command(bin, "team", "list")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "isn't signed in") {
		t.Fatalf("team list signed out: %v %s", err, out)
	}
}
