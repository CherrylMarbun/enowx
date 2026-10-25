package intercept

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppExecutableFindsTheBundlesProgram(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Chat.app")
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "MacOS", "Chat Helper"), nil, 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte("<dict>\n\t<key>CFBundleExecutable</key>\n\t<string>Chat Helper</string>\n</dict>"), 0o644)
	if got, want := AppExecutable(app), filepath.Join(app, "Contents", "MacOS", "Chat Helper"); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if got := AppExecutable("/usr/bin/electron"); got != "/usr/bin/electron" {
		t.Fatalf("a program: %s", got)
	}
}
