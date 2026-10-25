package android

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Project says what kind of app the project at dir is, the command that
// runs it on the device $ANDROID_SERIAL names, and its package when its
// files say it.
func Project(dir string) proto.AndroidProject {
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(dir, name))
		return string(b)
	}
	exists := func(name string) bool { return fileExists(filepath.Join(dir, name)) }
	pkgJSON := read("package.json")
	switch {
	case strings.Contains(read("pubspec.yaml"), "flutter:"):
		return proto.AndroidProject{Kind: "flutter", Command: `flutter run -d "$ANDROID_SERIAL"`, Package: gradlePackage(dir)}
	case strings.Contains(read("composer.json"), "nativephp/mobile"):
		return proto.AndroidProject{Kind: "nativephp", Command: "php artisan native:run android", Package: envValue(read(".env"), "NATIVEPHP_APP_ID")}
	case exists("capacitor.config.ts") || exists("capacitor.config.json") || exists("capacitor.config.js"):
		return proto.AndroidProject{Kind: "capacitor", Command: `npx cap run android --target "$ANDROID_SERIAL"`, Package: capacitorID(read("capacitor.config.json") + read("capacitor.config.ts") + read("capacitor.config.js"))}
	case strings.Contains(pkgJSON, `"expo"`):
		return proto.AndroidProject{Kind: "expo", Command: `npx expo run:android --device "$ANDROID_SERIAL"`, Package: expoPackage(read("app.json"))}
	case strings.Contains(pkgJSON, `"react-native"`):
		return proto.AndroidProject{Kind: "react-native", Command: `npx react-native run-android --deviceId "$ANDROID_SERIAL"`, Package: gradlePackage(dir)}
	}
	if pkg := gradlePackage(dir); pkg != "" || exists("app/build.gradle") || exists("app/build.gradle.kts") {
		cmd := "./gradlew installDebug"
		if pkg != "" {
			cmd += " && adb shell monkey -p " + pkg + " -c android.intent.category.LAUNCHER 1"
		}
		return proto.AndroidProject{Kind: "gradle", Command: cmd, Package: pkg}
	}
	return proto.AndroidProject{}
}

var appID = regexp.MustCompile(`applicationId\s*=?\s*["']([\w.]+)["']`)

// gradlePackage is the applicationId of the project's Android app.
func gradlePackage(dir string) string {
	for _, f := range []string{"android/app/build.gradle", "android/app/build.gradle.kts", "app/build.gradle", "app/build.gradle.kts"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		if m := appID.FindSubmatch(b); m != nil {
			return string(m[1])
		}
	}
	return ""
}

func expoPackage(appJSON string) string {
	var cfg struct {
		Expo struct {
			Android struct {
				Package string `json:"package"`
			} `json:"android"`
		} `json:"expo"`
	}
	json.Unmarshal([]byte(appJSON), &cfg)
	return cfg.Expo.Android.Package
}

var capID = regexp.MustCompile(`appId["']?\s*[:=]\s*["']([\w.]+)["']`)

func capacitorID(cfg string) string {
	if m := capID.FindStringSubmatch(cfg); m != nil {
		return m[1]
	}
	return ""
}

func envValue(env, key string) string {
	for _, line := range strings.Split(env, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}
