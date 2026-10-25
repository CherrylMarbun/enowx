// Package android finds and drives what a host has for Android: the SDK,
// its emulator and virtual devices, the devices adb sees, their logs, and
// how a project's app runs on one (docs/plans/mobile.md). The emulator
// runs on the host itself, never in a container: on macOS and Windows a
// container has no acceleration.
package android

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/enowx/enowx/internal/quietcmd"
)

// SDK is an Android SDK on this host.
type SDK struct {
	Root string
}

// exe is a program's name on this host.
func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// script is a command-line tool's name on this host (a .bat on Windows).
func script(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".bat"
	}
	return name
}

// DefaultRoot is where the SDK goes on this host when none is set:
// where Android Studio puts it.
func DefaultRoot() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Android", "sdk")
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "Android", "Sdk")
		}
	}
	return filepath.Join(home, "Android", "Sdk")
}

// FindSDK finds the host's SDK: ANDROID_HOME, ANDROID_SDK_ROOT, or the
// default place, where one of its tools is.
// An SDK not there yet goes where ANDROID_HOME (or ANDROID_SDK_ROOT) says
// when set, else to the default place.
func FindSDK() (SDK, bool) {
	for _, root := range []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT"), DefaultRoot()} {
		if root == "" {
			continue
		}
		s := SDK{Root: root}
		if fileExists(s.ADB()) || fileExists(s.SDKManager()) || fileExists(s.Emulator()) {
			return s, true
		}
	}
	for _, root := range []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT")} {
		if root != "" {
			return SDK{Root: root}, false
		}
	}
	return SDK{Root: DefaultRoot()}, false
}

func (s SDK) ADB() string      { return filepath.Join(s.Root, "platform-tools", exe("adb")) }
func (s SDK) Emulator() string { return filepath.Join(s.Root, "emulator", exe("emulator")) }
func (s SDK) SDKManager() string {
	return filepath.Join(s.Root, "cmdline-tools", "latest", "bin", script("sdkmanager"))
}
func (s SDK) AVDManager() string {
	return filepath.Join(s.Root, "cmdline-tools", "latest", "bin", script("avdmanager"))
}

// Env is what a session's programs need to find the SDK: ANDROID_HOME,
// its tools on PATH, and JAVA_HOME when enowX brought the JDK.
func (s SDK) Env(path string) []string {
	sep := string(os.PathListSeparator)
	dirs := []string{filepath.Join(s.Root, "platform-tools"), filepath.Join(s.Root, "emulator"), filepath.Join(s.Root, "cmdline-tools", "latest", "bin")}
	env := []string{"ANDROID_HOME=" + s.Root, "ANDROID_SDK_ROOT=" + s.Root}
	if jdk := JDKHome(); jdk != "" {
		env = append(env, "JAVA_HOME="+jdk)
		dirs = append(dirs, filepath.Join(jdk, "bin"))
	}
	return append(env, "PATH="+strings.Join(dirs, sep)+sep+path)
}

// JDKHome is the JDK enowX set up, "" when it didn't.
func JDKHome() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".enowx-app", "jdk")
	if runtime.GOOS == "darwin" {
		if fileExists(filepath.Join(dir, "Contents", "Home", "bin", exe("java"))) {
			return filepath.Join(dir, "Contents", "Home")
		}
	}
	if fileExists(filepath.Join(dir, "bin", exe("java"))) {
		return dir
	}
	return ""
}

// JavaVersion is the java a tool would run, "" for none.
func JavaVersion() string {
	java := "java"
	if jdk := JDKHome(); jdk != "" {
		java = filepath.Join(jdk, "bin", exe("java"))
	} else if jh := os.Getenv("JAVA_HOME"); jh != "" {
		java = filepath.Join(jh, "bin", exe("java"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := quietcmd.CommandContext(ctx, java, "-version").CombinedOutput()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line)
}

// toolEnv is the environment the SDK's own tools run with.
func (s SDK) toolEnv() []string {
	return append(os.Environ(), s.Env(os.Getenv("PATH"))...)
}

// command runs one of the SDK's tools.
func (s SDK) command(ctx context.Context, tool string, args ...string) *exec.Cmd {
	cmd := quietcmd.CommandContext(ctx, tool, args...)
	cmd.Env = s.toolEnv()
	return cmd
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
