package android

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDevices(t *testing.T) {
	out := "List of devices attached\nemulator-5554          device product:sdk_gphone64_x86_64 model:sdk_gphone64_x86_64 device:emu64xa\nR58M12345     unauthorized usb:1-1 transport_id:2\n192.168.1.5:41823 device product:panther model:Pixel_7 device:panther\n\n"
	got := parseDevices(out)
	if len(got) != 3 || got[0].Kind != "emulator" || got[1].State != "unauthorized" || got[1].Kind != "physical" || got[2].Name != "Pixel 7" {
		t.Fatalf("%+v", got)
	}
}

func TestNewestPackages(t *testing.T) {
	list := `Available Packages:
  build-tools;34.0.0   | 34.0.0 | Android SDK Build-Tools 34
  build-tools;35.0.1   | 35.0.1 | Android SDK Build-Tools 35.0.1
  build-tools;36.0.0   | 36.0.0 | Android SDK Build-Tools 36
  platforms;android-34 | 3 | Android SDK Platform 34
  platforms;android-35 | 2 | Android SDK Platform 35
  system-images;android-34;google_apis;x86_64 | 14 | Google APIs Intel x86_64 Atom System Image
  system-images;android-35;google_apis;x86_64 | 9  | Google APIs Intel x86_64 Atom System Image
  system-images;android-35;google_apis;arm64-v8a | 9 | Google APIs ARM 64 v8a System Image
`
	p, b, img := newestPackages(list, "x86_64")
	if p != "platforms;android-35" || b != "build-tools;35.0.1" || img != "system-images;android-35;google_apis;x86_64" {
		t.Fatalf("%q %q %q", p, b, img)
	}
}

func TestJavaMajor(t *testing.T) {
	for v, want := range map[string]int{`openjdk version "17.0.9" 2023-10-17`: 17, `java version "1.8.0_391"`: 8, `openjdk version "21" 2023-09-19`: 21, "": 0} {
		if got := javaMajor(v); got != want {
			t.Errorf("%q: %d", v, got)
		}
	}
}

func TestCmdlineToolsArchive(t *testing.T) {
	xml := `<sdk:sdk-repository xmlns:sdk="x"><remotePackage path="cmdline-tools;latest"><archives>
<archive><complete><url>commandlinetools-mac_x86_64-1_latest.zip</url></complete><host-os>macosx</host-os><host-arch>x64</host-arch></archive>
<archive><complete><url>commandlinetools-mac_arm64-1_latest.zip</url><checksum type="sha1">abc</checksum></complete><host-os>macosx</host-os><host-arch>aarch64</host-arch></archive>
<archive><complete><url>commandlinetools-linux-1_latest.zip</url></complete><host-os>linux</host-os></archive>
</archives></remotePackage></sdk:sdk-repository>`
	got, _, err := cmdlineToolsArchive([]byte(xml), "linux", "x64")
	if err != nil || got != "commandlinetools-linux-1_latest.zip" {
		t.Fatalf("%q %v", got, err)
	}
	got, sum, err := cmdlineToolsArchive([]byte(xml), "macosx", "aarch64")
	if err != nil || got != "commandlinetools-mac_arm64-1_latest.zip" || sum != "abc" {
		t.Fatalf("%q %q %v", got, sum, err)
	}
}

func TestParseLogLine(t *testing.T) {
	l, ok := parseLogLine("10-25 14:13:45.123  1234  1260 E AndroidRuntime: FATAL EXCEPTION: main")
	if !ok || l.PID != 1234 || l.Level != "E" || l.Tag != "AndroidRuntime" || l.Message != "FATAL EXCEPTION: main" {
		t.Fatalf("%+v %v", l, ok)
	}
}

func TestProjectKinds(t *testing.T) {
	for files, want := range map[string]string{
		"pubspec.yaml=name: app\nflutter:\n  uses-material-design: true":                 "flutter",
		`package.json={"dependencies":{"expo":"~52.0.0","react-native":"0.76"}}`:        "expo",
		`package.json={"dependencies":{"react-native":"0.76"}}`:                         "react-native",
		`capacitor.config.json={"appId":"com.example.app"}`:                             "capacitor",
		`composer.json={"require":{"nativephp/mobile":"^1"}}`:                           "nativephp",
		"app/build.gradle.kts=android { defaultConfig { applicationId = \"com.ex.a\" } }": "gradle",
	} {
		dir := t.TempDir()
		name, content, _ := strings.Cut(files, "=")
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
		got := Project(dir)
		if got.Kind != want || got.Command == "" {
			t.Errorf("%s: %+v", name, got)
		}
		if want == "capacitor" && got.Package != "com.example.app" || want == "gradle" && got.Package != "com.ex.a" {
			t.Errorf("%s: package %q", name, got.Package)
		}
	}
}
