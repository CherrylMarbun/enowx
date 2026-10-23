package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// The command line drives a real daemon: an agent spawned in a project,
// waited for until its hooks say it is done, read, typed into, listed and
// ended, as an agent would through $ENOWX_BIN.
func TestCommandLineDrivesTheDaemon(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "enowxd")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/enowx/enowx/cmd/enowxd").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	// A short home: the socket's path has a length limit.
	home, err := os.MkdirTemp("", "enx")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	paths := filepath.Join(tmp, "path")
	if err := os.MkdirAll(paths, 0o755); err != nil {
		t.Fatal(err)
	}
	// A "claude" that says what it was asked, reports working then done
	// through its hook, then echoes what it is sent.
	agent := "#!/bin/sh\nfor a in \"$@\"; do last=\"$a\"; done\necho \"asked: $last\"\n\"$ENOWX_BIN\" hook working\nsleep 1\n\"$ENOWX_BIN\" hook done\nwhile read -r line; do echo \"heard: $line\"; done\n"
	if err := os.WriteFile(filepath.Join(paths, "claude"), []byte(agent), 0o755); err != nil {
		t.Fatal(err)
	}
	// The daemon takes the login shell's PATH: a shell that keeps this one,
	// so the fake agent is the one found.
	shell := filepath.Join(tmp, "sh")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nfor a in \"$@\"; do last=\"$a\"; done\nexec /bin/sh -c \"$last\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "ENOWX_HOME="+home, "SHELL="+shell, "PATH="+paths+string(os.PathListSeparator)+os.Getenv("PATH"))
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
		for range 50 { // the daemon may still be starting
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
		return strings.TrimSpace(string(out))
	}
	project := t.TempDir()
	id := run("spawn", "--agent", "claude", "--project", project, "--name", "helper", "fix the tests")
	if len(id) != 12 {
		t.Fatalf("spawn printed %q", id)
	}
	if got := run("wait", "--timeout", "20s", "helper"); got != "done" {
		t.Fatalf("wait = %q", got)
	}
	if screen := run("read", id[:6]); !strings.Contains(screen, "asked: fix the tests") {
		t.Fatalf("read:\n%s", screen)
	}
	run("note", id, "fixing", "the", "tests")
	if out := run("ls"); !strings.Contains(out, "fixing the tests") {
		t.Fatalf("no note in ls:\n%s", out)
	}
	run("send", id, "and the docs")
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(run("read", id), "heard: ") {
		if time.Now().After(deadline) {
			t.Fatalf("never heard:\n%s", run("read", id))
		}
		time.Sleep(200 * time.Millisecond)
	}
	if screen := run("read", id); !strings.Contains(screen, "and the docs") {
		t.Fatalf("sent:\n%s", screen)
	}
	var list []proto.SessionInfo
	if err := json.Unmarshal([]byte(run("ls", "--json")), &list); err != nil || len(list) != 1 || list[0].Name != "helper" || list[0].Project != project {
		t.Fatalf("ls: %+v, %v", list, err)
	}
	if !strings.Contains(run("agents"), "claude") {
		t.Fatal("agents doesn't list the claude on PATH")
	}
	run("kill", id)
	if out := run("ls"); strings.Contains(out, id) {
		t.Fatalf("killed, still listed:\n%s", out)
	}

	// A pipeline: each stage's agent reports done through its hook, and
	// the next one starts, until all three are.
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	pid := run("pipeline", "--project", repo, "Add a search box")
	deadline = time.Now().Add(40 * time.Second)
	for {
		out := run("pipelines")
		if strings.Contains(out, pid+"  done  pipeline/add-a-search-box") {
			if strings.Count(out, ":done(") != 3 {
				t.Fatalf("pipelines:\n%s", out)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipeline never done:\n%s", out)
		}
		time.Sleep(300 * time.Millisecond)
	}
}
