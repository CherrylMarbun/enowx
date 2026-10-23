package setup

import (
	"os"
	"os/exec"
	"testing"
)

// TestSystemPackagesOnThisDistro runs the system packages step for real,
// on the Linux it runs on (scripts/test-distros.sh runs it in a container
// of each distribution enowX sets up), then scans again.
func TestSystemPackagesOnThisDistro(t *testing.T) {
	if os.Getenv("ENOWX_DISTRO_TEST") == "" {
		t.Skip("changes the system: run by scripts/test-distros.sh in containers")
	}
	before := Scan()
	if before.Packages == "" {
		t.Fatalf("no package manager found on %s %s", before.Distro, before.Release)
	}
	var m Manifest
	m.Git.Name, m.Git.Email = "Ada", "ada@example.com"
	steps := Plan(m, before)
	ran := 0
	for _, st := range steps {
		if st.ID != "packages" && st.ID != "git" {
			continue
		}
		cmd := exec.Command("sh", "-c", st.Script)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s on %s: %v\n%s", st.ID, before.Distro, err, st.Script)
		}
		ran++
	}
	if ran != 2 {
		t.Fatalf("ran %d steps of %+v", ran, steps)
	}
	after := Scan()
	for _, want := range []string{"git", "curl", "unzip", "make", "gcc"} {
		if _, ok := after.Tools[want]; !ok {
			t.Errorf("%s %s: no %s after the packages step", after.Distro, after.Release, want)
		}
	}
	if after.GitName != "Ada" || after.GitEmail != "ada@example.com" {
		t.Errorf("git user %q <%q>", after.GitName, after.GitEmail)
	}
	if left := Pending(Plan(m, after)); len(left) > 0 {
		t.Errorf("still to do on a second run: %+v", left)
	}
}
