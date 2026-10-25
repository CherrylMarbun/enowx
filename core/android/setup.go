package android

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// AVDName is the virtual phone Setup makes.
const AVDName = "enowX_Phone"

const (
	repositoryURL = "https://dl.google.com/android/repository/repository2-3.xml"
	downloadBase  = "https://dl.google.com/android/repository/"
	jdkURL        = "https://api.adoptium.net/v3/binary/latest/17/ga/%s/%s/jdk/hotspot/normal/eclipse"
)

// ImageABI is the system images' ABI that runs fast here: the host's own.
func ImageABI() string {
	if runtime.GOARCH == "arm64" {
		return "arm64-v8a"
	}
	return "x86_64"
}

// Setup brings this host what building and running Android apps need: a
// JDK (when none is 17 or newer), the SDK's command-line tools, its
// licenses (accepted by the user, in the terminal), platform-tools, the
// emulator, the newest platform, build-tools and system image, and a
// virtual phone. It writes what it does to out and reads the answers to
// the licenses from in.
func Setup(ctx context.Context, in io.Reader, out io.Writer) error {
	say := func(format string, args ...any) { fmt.Fprintf(out, "\x1b[1m"+format+"\x1b[0m\n", args...) }
	s, _ := FindSDK()
	say("Android SDK in %s", s.Root)

	if v := JavaVersion(); javaMajor(v) < 17 {
		say("A JDK 17 (Eclipse Temurin), into ~/.enowx-app/jdk")
		if err := installJDK(ctx, out); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "Java: %s\n", v)
	}

	if !fileExists(s.SDKManager()) {
		say("The SDK's command-line tools")
		if err := installCmdlineTools(ctx, s, out); err != nil {
			return err
		}
	}

	say("The SDK's licenses: read them, and answer y to accept")
	lic := s.command(ctx, s.SDKManager(), "--licenses")
	lic.Stdin, lic.Stdout, lic.Stderr = in, out, out
	if err := lic.Run(); err != nil {
		return fmt.Errorf("android: the licenses: %w", err)
	}

	say("Finding the newest packages")
	list, err := s.command(ctx, s.SDKManager(), "--list").Output()
	if err != nil {
		return fmt.Errorf("android: sdkmanager --list: %w", err)
	}
	platform, build, image := newestPackages(string(list), ImageABI())
	if platform == "" || image == "" {
		return errors.New("android: no platform or system image for this machine in the SDK's list")
	}
	pkgs := []string{"platform-tools", "emulator", platform, image}
	if build != "" {
		pkgs = append(pkgs, build)
	}
	say("Installing %s", strings.Join(pkgs, ", "))
	install := s.command(ctx, s.SDKManager(), pkgs...)
	install.Stdin, install.Stdout, install.Stderr = strings.NewReader(strings.Repeat("y\n", 20)), out, out
	if err := install.Run(); err != nil {
		return fmt.Errorf("android: installing the packages: %w", err)
	}

	if !slices.Contains(s.AVDs(), AVDName) {
		say("A virtual phone, %s", AVDName)
		image = strings.ReplaceAll(image, "/", ";") // as avdmanager names it
		create := s.command(ctx, s.AVDManager(), "create", "avd", "-n", AVDName, "-k", image, "-d", "pixel_7", "--force")
		create.Stdin, create.Stdout, create.Stderr = strings.NewReader("no\n"), out, out
		if err := create.Run(); err != nil {
			// Older tools don't know the Pixel 7.
			create = s.command(ctx, s.AVDManager(), "create", "avd", "-n", AVDName, "-k", image, "--force")
			create.Stdin, create.Stdout, create.Stderr = strings.NewReader("no\n"), out, out
			if err := create.Run(); err != nil {
				return fmt.Errorf("android: making the virtual phone: %w", err)
			}
		}
	}
	// A smaller data partition than the default 12 GB fits servers' disks.
	if err := setAVDConfig(AVDName, "disk.dataPartition.size", "4G"); err != nil {
		fmt.Fprintf(out, "Couldn't size the virtual phone's data: %v\n", err)
	}
	ok, why := s.Accel()
	if !ok {
		fmt.Fprintf(out, "\nThe emulator isn't accelerated here: %s\n", why)
	}
	say("Done. Start %s from the Android tool.", AVDName)
	return nil
}

// javaMajor is a java -version line's major version (17 for "17.0.9").
func javaMajor(v string) int {
	m := regexp.MustCompile(`version "(\d+)(?:\.(\d+))?`).FindStringSubmatch(v)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	if n == 1 && m[2] != "" { // 1.8
		n, _ = strconv.Atoi(m[2])
	}
	return n
}

// newestPackages picks from `sdkmanager --list` the newest stable
// Google APIs system image for abi, its platform and build-tools, as the
// list names them: packages' parts split by ";" in older tools, by "/"
// in newer ones.
func newestPackages(list, abi string) (platform, build, image string) {
	type pkg struct {
		id    string
		parts []string
	}
	var pkgs []pkg
	for _, line := range strings.Split(list, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || !strings.ContainsAny(f[0], ";/") {
			continue
		}
		pkgs = append(pkgs, pkg{f[0], strings.FieldsFunc(f[0], func(r rune) bool { return r == ';' || r == '/' })})
	}
	stable := regexp.MustCompile(`^android-(\d+)(?:\.(\d+))?$`)
	version := func(name string) (int, int, bool) {
		m := stable.FindStringSubmatch(name)
		if m == nil {
			return 0, 0, false
		}
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		return major, minor, true
	}
	bestMajor, bestMinor, apiName := -1, -1, ""
	for _, p := range pkgs {
		if len(p.parts) != 4 || p.parts[0] != "system-images" || p.parts[2] != "google_apis" || p.parts[3] != abi {
			continue
		}
		major, minor, ok := version(p.parts[1])
		if ok && (major > bestMajor || major == bestMajor && minor > bestMinor) {
			bestMajor, bestMinor, image, apiName = major, minor, p.id, p.parts[1]
		}
	}
	if image == "" {
		return "", "", ""
	}
	pMajor, pMinor := -1, -1
	for _, p := range pkgs {
		if len(p.parts) != 2 || p.parts[0] != "platforms" {
			continue
		}
		if p.parts[1] == apiName {
			platform = p.id
			break
		}
		if major, minor, ok := version(p.parts[1]); ok && major <= bestMajor && (major > pMajor || major == pMajor && minor > pMinor) {
			pMajor, pMinor, platform = major, minor, p.id
		}
	}
	buildRe := regexp.MustCompile(`^(\d+)\.\d+\.\d+$`)
	for _, p := range pkgs {
		if len(p.parts) != 2 || p.parts[0] != "build-tools" {
			continue
		}
		m := buildRe.FindStringSubmatch(p.parts[1])
		if m == nil {
			continue
		}
		if major, _ := strconv.Atoi(m[1]); major <= bestMajor && (build == "" || versionLess(build, p.id)) {
			build = p.id
		}
	}
	return platform, build, image
}

func versionLess(a, b string) bool {
	trim := func(s string) string {
		return strings.TrimPrefix(strings.TrimPrefix(s, "build-tools;"), "build-tools/")
	}
	pa, pb := strings.Split(trim(a), "."), strings.Split(trim(b), ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x < y
		}
	}
	return len(pa) < len(pb)
}

// installCmdlineTools downloads the newest command-line tools, as the
// SDK's repository lists them, into cmdline-tools/latest.
func installCmdlineTools(ctx context.Context, s SDK, out io.Writer) error {
	body, err := fetch(ctx, repositoryURL)
	if err != nil {
		return err
	}
	file, sum, err := cmdlineToolsArchive(body, hostOS(), hostArch())
	if err != nil {
		return err
	}
	zipPath := filepath.Join(os.TempDir(), file)
	if err := download(ctx, downloadBase+file, zipPath, out); err != nil {
		return err
	}
	defer os.Remove(zipPath)
	if err := checkSHA1(zipPath, sum); err != nil {
		return err
	}
	dest := filepath.Join(s.Root, "cmdline-tools", "latest")
	os.RemoveAll(dest)
	// The archive holds cmdline-tools/…: its contents go into latest.
	return unzip(zipPath, dest, "cmdline-tools/")
}

func hostOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macosx"
	case "windows":
		return "windows"
	}
	return "linux"
}

// cmdlineToolsArchive finds the newest cmdline-tools archive for host
// (and arch, aarch64 or x64, where the repository splits by it) in the
// repository's XML, with its SHA-1.
func cmdlineToolsArchive(xmlData []byte, host, arch string) (url, sha1sum string, err error) {
	var repo struct {
		Packages []struct {
			Path     string `xml:"path,attr"`
			Archives []struct {
				HostOS   string `xml:"host-os"`
				HostArch string `xml:"host-arch"`
				URL      string `xml:"complete>url"`
				Checksum string `xml:"complete>checksum"`
			} `xml:"archives>archive"`
		} `xml:"remotePackage"`
	}
	if err := xml.Unmarshal(xmlData, &repo); err != nil {
		return "", "", fmt.Errorf("android: the SDK's repository: %w", err)
	}
	for _, p := range repo.Packages {
		if p.Path != "cmdline-tools;latest" {
			continue
		}
		for _, a := range p.Archives {
			if a.HostOS == host && (a.HostArch == "" || a.HostArch == arch) {
				return a.URL, strings.TrimSpace(a.Checksum), nil
			}
		}
	}
	return "", "", errors.New("android: the SDK's repository lists no command-line tools for this system")
}

// hostArch is this machine in the repository's words.
func hostArch() string {
	if runtime.GOARCH == "arm64" {
		return "aarch64"
	}
	return "x64"
}

// installJDK downloads a JDK 17 into ~/.enowx-app/jdk.
func installJDK(ctx context.Context, out io.Writer) error {
	osName := map[string]string{"darwin": "mac", "windows": "windows"}[runtime.GOOS]
	if osName == "" {
		osName = "linux"
	}
	arch := map[string]string{"amd64": "x64", "arm64": "aarch64"}[runtime.GOARCH]
	if arch == "" {
		return fmt.Errorf("android: no JDK for %s", runtime.GOARCH)
	}
	home, _ := os.UserHomeDir()
	dest := filepath.Join(home, ".enowx-app", "jdk")
	tmp := filepath.Join(os.TempDir(), "enowx-jdk-archive")
	if err := download(ctx, fmt.Sprintf(jdkURL, osName, arch), tmp, out); err != nil {
		return err
	}
	defer os.Remove(tmp)
	os.RemoveAll(dest)
	if runtime.GOOS == "windows" {
		return unzip(tmp, dest, "*")
	}
	return untar(tmp, dest)
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("android: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("android: %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// download saves url at path, saying how far it got.
func download(ctx context.Context, url, path string, out io.Writer) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("android: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("android: %s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var done int64
	last := time.Now()
	buf := make([]byte, 256<<10)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if time.Since(last) > time.Second {
				last = time.Now()
				if resp.ContentLength > 0 {
					fmt.Fprintf(out, "\r  %d of %d MB", done>>20, resp.ContentLength>>20)
				} else {
					fmt.Fprintf(out, "\r  %d MB", done>>20)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("android: downloading %s: %w", url, err)
		}
	}
	fmt.Fprintf(out, "\r  %d MB\n", done>>20)
	return nil
}

// unzip extracts the zip at src into dest, stripping prefix from names
// ("*" strips the first folder, whatever it is).
func unzip(src, dest, prefix string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		name := f.Name
		if prefix == "*" {
			if _, rest, ok := strings.Cut(name, "/"); ok {
				name = rest
			}
		} else {
			name = strings.TrimPrefix(name, prefix)
		}
		if name == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(filepath.Separator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0o755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0o600)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(w, rc)
		rc.Close()
		w.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// untar extracts a .tar.gz into dest, stripping its first folder.
func untar(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		_, rest, ok := strings.Cut(h.Name, "/")
		if !ok || rest == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(rest))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(filepath.Separator)) {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0o755)
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0o755)
			os.Symlink(h.Linkname, target)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0o755)
			w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode)|0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, tr)
			w.Close()
			if err != nil {
				return err
			}
		}
	}
}

// checkSHA1 checks a download against the SHA-1 its repository gives.
func checkSHA1(path, want string) error {
	if want == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("android: %s is damaged: its SHA-1 is %s, not %s", filepath.Base(path), got, want)
	}
	return nil
}

// setAVDConfig sets key in virtual device avd's config.ini.
func setAVDConfig(avd, key, value string) error {
	dir := os.Getenv("ANDROID_AVD_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".android", "avd")
	}
	file := filepath.Join(dir, avd+".avd", "config.ini")
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	set := false
	for i, l := range lines {
		if k, _, ok := strings.Cut(l, "="); ok && strings.TrimSpace(k) == key {
			lines[i], set = key+"="+value, true
		}
	}
	if !set {
		lines = append(lines, key+"="+value)
	}
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
