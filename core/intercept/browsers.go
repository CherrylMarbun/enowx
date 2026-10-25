package intercept

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/enowx/enowx/internal/quietcmd"
)

// Program is a browser found on a host, to start with its traffic
// caught.
type Program struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Firefox bool   `json:"firefox,omitempty"`
}

type knownBrowser struct {
	id, name string
	firefox  bool
	commands []string            // on PATH (Linux)
	paths    map[string][]string // by GOOS, absolute
}

var knownBrowsers = []knownBrowser{
	{id: "chrome", name: "Google Chrome", commands: []string{"google-chrome", "google-chrome-stable"}, paths: map[string][]string{
		"darwin":  {"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
		"windows": {`%ProgramFiles%\Google\Chrome\Application\chrome.exe`, `%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`, `%LocalAppData%\Google\Chrome\Application\chrome.exe`},
	}},
	{id: "chromium", name: "Chromium", commands: []string{"chromium", "chromium-browser"}, paths: map[string][]string{
		"darwin": {"/Applications/Chromium.app/Contents/MacOS/Chromium"},
	}},
	{id: "edge", name: "Microsoft Edge", commands: []string{"microsoft-edge", "microsoft-edge-stable"}, paths: map[string][]string{
		"darwin":  {"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"},
		"windows": {`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`, `%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`},
	}},
	{id: "brave", name: "Brave", commands: []string{"brave-browser", "brave"}, paths: map[string][]string{
		"darwin":  {"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"},
		"windows": {`%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe`},
	}},
	{id: "firefox", name: "Firefox", firefox: true, commands: []string{"firefox"}, paths: map[string][]string{
		"darwin":  {"/Applications/Firefox.app/Contents/MacOS/firefox"},
		"windows": {`%ProgramFiles%\Mozilla Firefox\firefox.exe`, `%ProgramFiles(x86)%\Mozilla Firefox\firefox.exe`},
	}},
}

// Browsers lists the browsers found on this host.
func Browsers() []Program {
	var out []Program
	for _, b := range knownBrowsers {
		path := ""
		for _, p := range b.paths[runtime.GOOS] {
			p = os.ExpandEnv(strings.NewReplacer("%ProgramFiles%", "${ProgramFiles}", "%ProgramFiles(x86)%", "${ProgramFiles(x86)}", "%LocalAppData%", "${LOCALAPPDATA}").Replace(p))
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
		if path == "" && runtime.GOOS != "windows" {
			for _, c := range b.commands {
				if p, err := exec.LookPath(c); err == nil {
					path = p
					break
				}
			}
		}
		if path != "" {
			out = append(out, Program{ID: b.id, Name: b.name, Path: path, Firefox: b.firefox})
		}
	}
	return out
}

// ChromiumArgs start a Chromium-based browser or an Electron app on a
// fresh profile in dir, sending everything through the proxy on port, and
// trusting the CA's key (spki) for its certificates only.
func ChromiumArgs(dir string, port int, spki, startURL string) []string {
	args := []string{
		"--user-data-dir=" + dir,
		"--proxy-server=http://127.0.0.1:" + strconv.Itoa(port),
		// Loopback isn't bypassed: dev servers are caught too.
		"--proxy-bypass-list=<-loopback>",
		"--ignore-certificate-errors-spki-list=" + spki,
		"--no-first-run", "--no-default-browser-check", "--disable-background-networking",
		"--test-type", // no banner about the flag above
	}
	if startURL != "" {
		args = append(args, startURL)
	}
	return args
}

// FirefoxProfile writes a Firefox profile in dir whose proxy is the
// interceptor on port, and returns Firefox's arguments for it. The CA is
// added to the profile's certificates with NSS's certutil when the host
// has it; without, the note says so.
func FirefoxProfile(dir string, port int, caFile, startURL string) (args []string, note string, err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	prefs := fmt.Sprintf(`user_pref("network.proxy.type", 1);
user_pref("network.proxy.http", "127.0.0.1");
user_pref("network.proxy.http_port", %d);
user_pref("network.proxy.ssl", "127.0.0.1");
user_pref("network.proxy.ssl_port", %d);
user_pref("network.proxy.no_proxies_on", "");
user_pref("network.proxy.allow_hijacking_localhost", true);
user_pref("browser.shell.checkDefaultBrowser", false);
user_pref("browser.aboutwelcome.enabled", false);
user_pref("datareporting.policy.dataSubmissionEnabled", false);
user_pref("toolkit.telemetry.reportingpolicy.firstRun", false);
user_pref("security.enterprise_roots.enabled", true);
`, port, port)
	if err := os.WriteFile(filepath.Join(dir, "user.js"), []byte(prefs), 0o600); err != nil {
		return nil, "", err
	}
	// Windows has a certutil of its own, not NSS's.
	if certutil, err := exec.LookPath("certutil"); err == nil && !windowsCertutil(certutil) {
		quietcmd.Command(certutil, "-N", "-d", "sql:"+dir, "--empty-password").Run()
		if out, err := quietcmd.Command(certutil, "-A", "-d", "sql:"+dir, "-n", "enowX traffic CA", "-t", "C,,", "-i", caFile).CombinedOutput(); err != nil {
			note = "Firefox won't trust enowX's CA: certutil failed (" + strings.TrimSpace(string(out)) + ")"
		}
	} else {
		note = "Firefox will warn on HTTPS sites: install NSS's certutil (libnss3-tools, nss-tools or nss) so enowX can add its CA to the profile"
	}
	args = []string{"-profile", dir, "-no-remote", "-new-instance"}
	if startURL != "" {
		args = append(args, startURL)
	}
	return args, note, nil
}

// windowsCertutil reports whether path is Windows' own certutil, which
// takes other arguments than NSS's.
func windowsCertutil(path string) bool {
	return runtime.GOOS == "windows" && strings.Contains(strings.ToLower(path), `\windows\system32\`)
}
