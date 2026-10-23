package daemon

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

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
		{"branch", "fix/old-bug"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// Resolve symlinks (macOS /var -> /private/var) so paths compare equal.
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

func TestSessionPlacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	repo := gitRepo(t)
	folder := t.TempDir()
	if _, err := c.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddProject(folder); err != nil {
		t.Fatal(err)
	}
	projects, err := c.Projects()
	if err != nil || len(projects) != 2 || !projects[0].IsGit || projects[1].IsGit {
		t.Fatalf("projects = %+v, %v", projects, err)
	}
	detail, err := c.InspectProject(repo)
	if err != nil || detail.Branch != "main" || len(detail.Branches) != 2 {
		t.Fatalf("detail = %+v, %v", detail, err)
	}

	shell := []string{"sh", "-c", "sleep 30"}
	cases := []struct {
		name         string
		params       proto.SessionCreateParams
		wantWorktree bool
		wantDir      string
		wantBranch   string
	}{
		{"project folder", proto.SessionCreateParams{Project: repo, Command: shell}, false, repo, "main"},
		{"new branch", proto.SessionCreateParams{Project: repo, BranchMode: proto.BranchNew, Branch: "feat/search", Command: shell}, true, "", "feat/search"},
		{"existing branch", proto.SessionCreateParams{Project: repo, BranchMode: proto.BranchExisting, Branch: "fix/old-bug", Command: shell}, true, "", "fix/old-bug"},
		{"current branch", proto.SessionCreateParams{Project: repo, BranchMode: proto.BranchExisting, Branch: "main", Command: shell}, false, repo, "main"},
		{"plain folder", proto.SessionCreateParams{Project: folder, Command: shell}, false, folder, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := c.CreateSession(tc.params)
			if err != nil {
				t.Fatal(err)
			}
			if info.Worktree != tc.wantWorktree || info.Branch != tc.wantBranch || info.Project != tc.params.Project {
				t.Fatalf("info = %+v", info)
			}
			if tc.wantDir != "" && info.Dir != tc.wantDir {
				t.Fatalf("dir = %s, want %s", info.Dir, tc.wantDir)
			}
			if tc.wantWorktree {
				head, err := exec.Command("git", "-C", info.Dir, "branch", "--show-current").Output()
				if err != nil || strings.TrimSpace(string(head)) != tc.wantBranch {
					t.Fatalf("worktree branch = %q, %v", head, err)
				}
			}
		})
	}

	for _, bad := range []proto.SessionCreateParams{
		{Project: repo, BranchMode: proto.BranchNew, Branch: "fix/old-bug"}, // exists
		{Project: repo, BranchMode: proto.BranchExisting, Branch: "nope"},   // missing
		{Project: folder, BranchMode: proto.BranchNew, Branch: "feat/x"},    // not git
		{Project: repo, BranchMode: proto.BranchNew, Branch: "bad..name"},   // invalid
	} {
		if _, err := c.CreateSession(bad); err == nil {
			t.Errorf("CreateSession(%+v) succeeded", bad)
		}
	}
}

func TestPromptIsTypedIntoAgentsWithoutPromptArgument(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	// "custom" agents get the prompt typed once their screen settles.
	info, err := c.CreateSession(proto.SessionCreateParams{
		Agent: "custom", Command: []string{"sh", "-c", "echo ready; read line; echo got:$line; sleep 30"},
		Dir: os.TempDir(), Prompt: "hello agent", Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	watch(st).waitFor(t, "got:hello agent")
}

func TestFirstTypedLineBecomesThePrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	updates := make(chan proto.SessionInfo, 64)
	c := connect(t, srv, func(ev proto.Event) {
		var info proto.SessionInfo
		if ev.Name == proto.EventSessionUpdated && json.Unmarshal(ev.Data, &info) == nil {
			updates <- info
		}
	})
	info, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"cat"}, Dir: os.TempDir(), Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	// An arrow key, a typo fixed with backspace, then Enter.
	if _, err := st.Write([]byte("\x1b[Afix the logn\x7fin bug\r")); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case u := <-updates:
			if u.ID == info.ID && u.Prompt != "" {
				if u.Prompt != "fix the login bug" {
					t.Fatalf("prompt = %q", u.Prompt)
				}
				return
			}
		case <-deadline:
			t.Fatal("no prompt update")
		}
	}
}

// Sign-in runs the agent's sign-in command in the project folder, never
// in a new worktree, and names the session after it.
func TestSignInRunsInTheProjectFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	fake := "#!/bin/sh\necho \"signing-in:$*\"\n"
	if err := os.WriteFile(filepath.Join(bin, "kiro-cli"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(filepath.ListSeparator)+os.Getenv("PATH"))
	srv := startServer(t)
	c := connect(t, srv, nil)
	repo := gitRepo(t)
	if _, err := c.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	info, err := c.CreateSession(proto.SessionCreateParams{
		Project: repo, Agent: "kiro", SignIn: true, BranchMode: proto.BranchNew, Branch: "feat/x", Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Worktree || info.Dir != repo || info.Name != "Sign in to Kiro CLI" {
		t.Fatalf("info = %+v", info)
	}
	st, err := c.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	watch(st).waitFor(t, "signing-in:login --use-device-flow")

	if _, err := c.CreateSession(proto.SessionCreateParams{Project: repo, Agent: "custom", SignIn: true}); err == nil {
		t.Fatal("sign-in to an agent without one succeeded")
	}
}

// Switching mode cycles the agent with its key until its screen says the
// mode asked for.
func TestSwitchModeCyclesUntilTheScreenSaysIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	fake := `#!/bin/sh
i=0
show() {
	case $((i % 5)) in
	0) m="auto mode on" ;; 1) m="manual mode on" ;; 2) m="accept edits on" ;;
	3) m="plan mode on" ;; *) m="bypass permissions on" ;;
	esac
	printf '\r\nmode: %s' "$m"
}
show
stty raw -echo
while dd bs=3 count=1 >/dev/null 2>&1; do i=$((i + 1)); show; done
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(filepath.ListSeparator)+os.Getenv("PATH"))
	srv := startServer(t)
	c := connect(t, srv, nil)
	info, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: os.TempDir(), Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	s := watch(st)
	s.waitFor(t, "auto mode on")
	if err := c.SwitchMode(info.ID, "plan"); err != nil {
		t.Fatal(err)
	}
	s.waitFor(t, "plan mode on")
	time.Sleep(4 * modeStep)
	sess, _ := srv.sessions.Get(info.ID)
	if text := sess.Terminal().Text(); strings.Contains(text, "bypass permissions on") {
		t.Fatalf("cycled past plan:\n%s", text)
	}
	if err := c.SwitchMode(info.ID, "nonsense"); err == nil {
		t.Fatal("switched to an unknown mode")
	}
}
