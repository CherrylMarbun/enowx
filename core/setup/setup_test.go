package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// What this device has, against what a fresh server has: the steps say
// what each brings and whether the server has it.
func TestPlanAgainstAServer(t *testing.T) {
	local := proto.SetupScan{OS: "darwin", Packages: "brew", GitName: "Ada", GitEmail: "ada@x.dev", GitHub: true,
		Tools: map[string]string{"node": "22.11.0", "go": "1.27.3", "claude": "2.1.309", "git": "2.45.0"}}
	server := proto.SetupScan{OS: "linux", Distro: "debian", Packages: "apt-get",
		Tools: map[string]string{"git": "2.39.5", "curl": "7.88.1", "go": "1.27.1", "node": "18.19.0", "python3": "3.13.5"}}
	local.Tools["python3"] = "3.9.6" // macOS's own: older than the server's
	m := FromScan(local)
	m.SignIn = []string{"claude"}
	steps := Plan(m, server)
	byID := map[string]Step{}
	var order []string
	for _, s := range steps {
		byID[s.ID] = s
		order = append(order, s.ID)
	}
	want := "packages mise tool:node tool:python tool:go git gh gh-auth agent:claude signin:claude"
	if got := strings.Join(order, " "); got != want {
		t.Fatalf("steps %s\nwant  %s", got, want)
	}
	for id, done := range map[string]bool{"packages": false, "mise": false, "tool:node": false, "tool:python": true, "tool:go": true, "git": false, "agent:claude": false} {
		if byID[id].Done != done {
			t.Errorf("%s done = %v (%s)", id, byID[id].Done, byID[id].Detail)
		}
	}
	if s := byID["tool:node"]; s.Detail != "has 18.19.0" || !strings.Contains(s.Script, "use -g node@22.11.0") {
		t.Errorf("node step %+v", s)
	}
	if !byID["packages"].Asks || !strings.Contains(byID["packages"].Script, "sudo apt-get") {
		t.Errorf("packages step %+v", byID["packages"])
	}
	if len(Pending(steps)) != len(steps)-2 {
		t.Errorf("pending %d of %d", len(Pending(steps)), len(steps))
	}
}

// The script runs each step, a failure going on to the next, and records
// how each went.
func TestScriptRecordsEachStep(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	status := filepath.Join(dir, "status")
	script := Script([]Step{
		{ID: "a", Title: "First", Script: "true"},
		{ID: "b", Title: "Fails", Script: "false\necho not reached > " + filepath.Join(dir, "b")},
		{ID: "c", Title: "It's after", Script: "echo ran > " + filepath.Join(dir, "c")},
	}, status)
	if out, err := exec.Command("sh", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	data, _ := os.ReadFile(status)
	got := Status(data)
	if got["a"] != StateOK || got["b"] != StateFailed || got["c"] != StateOK {
		t.Fatalf("status %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "b")); err == nil {
		t.Error("a failed step went on")
	}
}

func TestManifestRoundTrip(t *testing.T) {
	m := FromScan(proto.SetupScan{GitName: "Ada", Tools: map[string]string{"node": "22.11.0", "claude": "2.1"}})
	data, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(data)
	if err != nil || back.Tools["node"] != "22.11.0" || back.Git.Name != "Ada" || len(back.Agents) != 1 {
		t.Fatalf("decoded %+v, %v\n%s", back, err, data)
	}
	if _, err := Decode([]byte(`agents = ["nope"]`)); err == nil {
		t.Fatal("decoded an unknown agent")
	}
}

// A scan finds git where it is installed, with its version.
func TestScanFindsGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	s := Scan()
	if s.Tools["git"] == "" || s.OS != runtime.GOOS {
		t.Fatalf("scan %+v", s)
	}
	t.Logf("%+v", s)
}
