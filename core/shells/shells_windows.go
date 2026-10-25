//go:build windows

package shells

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Login is the shell a terminal gets until the user picks another:
// PowerShell, the newer one when installed.
func Login() string {
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return os.Getenv("COMSPEC")
}

func candidates() []proto.ShellInfo {
	var out []proto.ShellInfo
	look := func(file, name string) {
		if path, err := exec.LookPath(file); err == nil {
			out = append(out, proto.ShellInfo{Name: name, Path: path})
		}
	}
	look("pwsh.exe", "PowerShell")
	look("powershell.exe", "Windows PowerShell")
	if cmd := os.Getenv("COMSPEC"); cmd != "" {
		out = append(out, proto.ShellInfo{Name: "Command Prompt", Path: cmd})
	}
	// Git for Windows' bash, beside its git.
	if git, err := exec.LookPath("git.exe"); err == nil {
		root := filepath.Dir(filepath.Dir(git)) // …\Git\cmd\git.exe
		out = append(out, proto.ShellInfo{Name: "Git Bash", Path: filepath.Join(root, "bin", "bash.exe")})
	}
	for _, dir := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432")} {
		if dir != "" {
			out = append(out, proto.ShellInfo{Name: "Git Bash", Path: filepath.Join(dir, "Git", "bin", "bash.exe")})
		}
	}
	look("wsl.exe", "WSL")
	return out
}

// Argv starts the shell at path as a terminal's.
func Argv(path string) []string {
	switch strings.ToLower(filepath.Base(path)) {
	case "pwsh.exe", "powershell.exe":
		return []string{path, "-NoLogo"}
	case "bash.exe":
		return []string{path, "--login", "-i"}
	}
	return []string{path}
}
