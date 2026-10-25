package setup

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/display"
	"github.com/enowx/enowx/proto"
)

// Step is one thing a setup does on a host: what it brings, whether the
// host has it already, and the shell that brings it, which checks first,
// so running it again changes nothing.
type Step struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail"` // what the host has now
	Done   bool   `json:"done"`   // the host has it
	Script string `json:"script"`
	// Asks says the step asks the user in the terminal: a sudo password,
	// a code to enter in a browser.
	Asks bool `json:"asks,omitempty"`
}

// misePath finds mise, which the mise step installs in ~/.local/bin.
const misePath = `"$(command -v mise || echo "$HOME/.local/bin/mise")"`

// basePackages are the system packages the other steps need, by package
// manager: git, curl, unzip, and a compiler for the tools that build.
var basePackages = map[string][]string{
	"apt-get": {"git", "curl", "unzip", "ca-certificates", "build-essential"},
	"dnf":     {"git", "curl", "unzip", "gcc", "make"},
	"zypper":  {"git", "curl", "unzip", "gcc", "make"},
	"pacman":  {"git", "curl", "unzip", "base-devel"},
	// Alpine has no bash, which agents' installers run.
	"apk":  {"git", "curl", "unzip", "bash", "build-base", "ca-certificates"},
	"brew": {"git"},
}

// Plan lists the steps that bring the host target scanned to m, in the
// order they run: system packages, mise and the toolchains, git, gh, the
// agents, then what desktop apps need.
func Plan(m Manifest, target proto.SetupScan) []Step {
	var steps []Step
	has := func(program string) bool { _, ok := target.Tools[program]; return ok }
	sudo := "sudo "
	switch {
	case target.Root:
		sudo = ""
	case !has("sudo") && has("doas"):
		sudo = "doas "
	}

	if pkgs := basePackages[target.Packages]; len(pkgs) > 0 {
		needed := []string{"git", "curl", "unzip", "make"}
		if target.Packages == "brew" {
			needed = []string{"git"} // macOS has the others
		}
		var missing []string
		for _, p := range needed {
			if !has(p) {
				missing = append(missing, p)
			}
		}
		steps = append(steps, Step{
			ID: "packages", Title: "System packages: " + strings.Join(pkgs, ", "),
			Detail: lacking(missing), Done: len(missing) == 0,
			Script: installPackages(target.Packages, sudo, pkgs), Asks: !target.Root && target.Packages != "brew",
		})
	}

	if len(m.Tools) > 0 {
		steps = append(steps, Step{
			ID: "mise", Title: "mise, which installs the toolchains", Detail: present(target.Tools["mise"], has("mise")), Done: has("mise"),
			Script: `command -v mise >/dev/null || [ -x "$HOME/.local/bin/mise" ] || curl -fsSL https://mise.run | sh
line='export PATH="$HOME/.local/share/mise/shims:$HOME/.local/bin:$PATH"'
for f in "$HOME/.profile" "$HOME/.bashrc" "$HOME/.zshrc"; do
  [ -f "$f" ] || [ "$f" = "$HOME/.profile" ] || continue
  grep -qsF "mise/shims" "$f" || printf '\n%s\n' "$line" >> "$f"
done`,
		})
	}
	for _, n := range miseNames {
		want, ok := m.Tools[n.tool]
		if !ok {
			continue
		}
		have := target.Tools[n.program]
		detail := present(have, has(n.program))
		newer := newerThan(have, want)
		if newer {
			// An older one here (macOS's own Python, say) isn't worth
			// going back to.
			detail += ", newer"
		}
		steps = append(steps, Step{
			ID: "tool:" + n.tool, Title: titleCase(n.tool) + " " + want, Detail: detail,
			Done:   sameMinor(have, want) || newer,
			Script: fmt.Sprintf(`%s use -g %s@%s`, misePath, n.tool, want),
		})
	}

	if m.Git.Name != "" || m.Git.Email != "" {
		var cmds []string
		if m.Git.Name != "" {
			cmds = append(cmds, "git config --global user.name "+quote(m.Git.Name))
		}
		if m.Git.Email != "" {
			cmds = append(cmds, "git config --global user.email "+quote(m.Git.Email))
		}
		steps = append(steps, Step{
			ID: "git", Title: "Git user: " + strings.TrimSpace(m.Git.Name+" <"+m.Git.Email+">"),
			Detail: present(strings.TrimSpace(target.GitName+" "+target.GitEmail), target.GitName != "" || target.GitEmail != ""),
			Done:   target.GitName == m.Git.Name && target.GitEmail == m.Git.Email,
			Script: strings.Join(cmds, "\n"),
		})
	}

	if m.GitHub {
		steps = append(steps, Step{
			ID: "gh", Title: "GitHub CLI (gh)", Detail: present(target.Tools["gh"], has("gh")), Done: has("gh"),
			Script: `command -v gh >/dev/null || ` + misePath + ` use -g gh@latest`,
		}, Step{
			ID: "gh-auth", Title: "Sign gh in to GitHub", Detail: signedIn(target.GitHub), Done: target.GitHub, Asks: true,
			Script: `gh auth status >/dev/null 2>&1 || gh auth login --git-protocol https --web
gh auth setup-git`,
		})
	}

	for _, id := range m.Agents {
		p, ok := agent.Find(id)
		if !ok || p.Install == "" {
			continue
		}
		steps = append(steps, Step{
			ID: "agent:" + id, Title: p.Name, Detail: present(target.Tools[p.Command], has(p.Command)), Done: has(p.Command),
			Script: "command -v " + p.Command + " >/dev/null || { " + p.Install + "; }",
		})
	}
	for _, id := range m.SignIn {
		p, ok := agent.Find(id)
		if !ok || len(p.SignIn) == 0 {
			continue
		}
		steps = append(steps, Step{
			ID: "signin:" + id, Title: "Sign " + p.Name + " in", Detail: "a code to enter in a browser", Asks: true,
			Script: strings.Join(append(envPrefix(p.SignInEnv), quoteAll(p.SignIn)...), " "),
		})
	}

	if m.Display && target.OS == "linux" {
		if cmd := display.InstallFor(target.Packages, target.Root); cmd != "" {
			steps = append(steps, Step{
				ID: "display", Title: "Xpra, for desktop apps", Detail: present(target.Tools["xpra"], has("xpra")), Done: has("xpra"),
				Script: "command -v xpra >/dev/null || " + cmd, Asks: !target.Root,
			})
		}
	}
	if m.Android {
		// enowX's own setup: the daemon's program sets Android up, the
		// SDK's licenses read and accepted in the terminal.
		steps = append(steps, Step{
			ID: "android", Title: "Android SDK, an emulator and a virtual phone", Detail: present(target.Tools["adb"], has("adb")), Done: has("adb"),
			Script: `"${ENOWX_BIN:-enowx-app}" android setup`, Asks: true,
		})
	}
	return steps
}

func installPackages(pm, sudo string, pkgs []string) string {
	list := strings.Join(pkgs, " ")
	switch pm {
	case "apt-get":
		return sudo + "apt-get update && " + sudo + "env DEBIAN_FRONTEND=noninteractive apt-get install -y " + list
	case "dnf":
		return sudo + "dnf install -y " + list
	case "zypper":
		return sudo + "zypper install -y " + list
	case "pacman":
		return sudo + "pacman -Sy --needed --noconfirm " + list
	case "apk":
		return sudo + "apk add --no-cache " + list
	case "brew":
		return "brew install " + list
	}
	return ""
}

// sameMinor reports whether version have is want's, to its minor: a patch
// apart is the same toolchain.
func sameMinor(have, want string) bool {
	if have == "" {
		return false
	}
	cut := func(v string) string {
		parts := strings.SplitN(v, ".", 3)
		return strings.Join(parts[:min(len(parts), 2)], ".")
	}
	return cut(have) == cut(want)
}

// newerThan reports whether version have is newer than want.
func newerThan(have, want string) bool {
	if have == "" || want == "" {
		return false
	}
	h, w := strings.Split(have, "."), strings.Split(want, ".")
	for i := 0; i < max(len(h), len(w)); i++ {
		var a, b int
		if i < len(h) {
			a, _ = strconv.Atoi(h[i])
		}
		if i < len(w) {
			b, _ = strconv.Atoi(w[i])
		}
		if a != b {
			return a > b
		}
	}
	return false
}

func present(version string, ok bool) string {
	switch {
	case !ok:
		return "missing"
	case version == "":
		return "installed"
	}
	return "has " + version
}

func lacking(missing []string) string {
	if len(missing) == 0 {
		return "installed"
	}
	return "missing " + strings.Join(missing, ", ")
}

func signedIn(ok bool) string {
	if ok {
		return "signed in"
	}
	return "not signed in"
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// quote quotes s for a POSIX shell.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func quoteAll(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = quote(a)
	}
	return out
}

func envPrefix(env []string) []string {
	if len(env) == 0 {
		return nil
	}
	return append([]string{"env"}, quoteAll(env)...)
}

// Pending are the steps a host still needs.
func Pending(steps []Step) []Step {
	return slices.DeleteFunc(slices.Clone(steps), func(s Step) bool { return s.Done })
}
