package intercept

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Trust is where a host keeps the CA for tools to trust: the CA alone,
// and a bundle of the host's own roots with the CA, for tools whose
// setting replaces their roots rather than adds to them.
type Trust struct {
	CA     string
	Bundle string
}

// systemRoots are where Unix systems keep their roots as one file.
var systemRoots = []string{
	"/etc/ssl/certs/ca-certificates.crt", // Debian, Ubuntu, Alpine, Arch
	"/etc/pki/tls/certs/ca-bundle.crt",   // Fedora, RHEL
	"/etc/ssl/ca-bundle.pem",             // openSUSE
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
	"/etc/ssl/cert.pem", // macOS, Alpine
}

// WriteTrust writes the CA and the bundle into dir.
func WriteTrust(dir string, ca *CA) (Trust, error) {
	t := Trust{CA: filepath.Join(dir, "ca.pem"), Bundle: filepath.Join(dir, "bundle.pem")}
	var bundle []byte
	for _, f := range systemRoots {
		if b, err := os.ReadFile(f); err == nil && len(b) > 0 {
			bundle = append(b, '\n')
			break
		}
	}
	bundle = append(bundle, ca.PEM...)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return t, fmt.Errorf("intercept: %w", err)
	}
	if err := os.WriteFile(t.CA, ca.PEM, 0o644); err != nil {
		return t, fmt.Errorf("intercept: %w", err)
	}
	if err := os.WriteFile(t.Bundle, bundle, 0o644); err != nil {
		return t, fmt.Errorf("intercept: %w", err)
	}
	return t, nil
}

// Env is the environment that sends a program's HTTP through proxy (an
// http:// URL) and has it trust the CA: the proxy variables in both
// cases, which tools read one or the other of, and each language's own
// setting for extra roots.
func Env(proxy string, t Trust) []string {
	env := []string{
		"HTTP_PROXY=" + proxy, "HTTPS_PROXY=" + proxy, "http_proxy=" + proxy, "https_proxy=" + proxy,
		"ALL_PROXY=" + proxy, "all_proxy=" + proxy,
		"GLOBAL_AGENT_HTTP_PROXY=" + proxy,
		"NODE_USE_ENV_PROXY=1", // Node 24+ reads the proxy variables
		"NODE_EXTRA_CA_CERTS=" + t.CA,
		"DENO_CERT=" + t.CA,
		"REQUESTS_CA_BUNDLE=" + t.Bundle,
		"CURL_CA_BUNDLE=" + t.Bundle,
		"GIT_SSL_CAINFO=" + t.Bundle,
		"PIP_CERT=" + t.Bundle,
		"AWS_CA_BUNDLE=" + t.Bundle,
		"CARGO_HTTP_CAINFO=" + t.Bundle,
		"HEX_CACERTS_PATH=" + t.Bundle,
		"ENOWX_TRAFFIC=1",
	}
	if runtime.GOOS != "windows" {
		// Go, OpenSSL, Ruby and Python's ssl read it on Unix; it
		// replaces the system's roots, hence the bundle.
		env = append(env, "SSL_CERT_FILE="+t.Bundle)
	}
	return env
}

// EnvNames are the variables Env sets, to undo them.
func EnvNames() []string {
	names := map[string]bool{}
	for _, kv := range Env("", Trust{}) {
		k, _, _ := strings.Cut(kv, "=")
		names[k] = true
	}
	names["SSL_CERT_FILE"] = true
	out := make([]string, 0, len(names))
	for k := range names {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ShellExports is env as sh export lines.
func ShellExports(env []string) string {
	var b strings.Builder
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		fmt.Fprintf(&b, "export %s=%s\n", k, shellQuote(v))
	}
	return b.String()
}

// ShellUnset undoes ShellExports.
func ShellUnset() string { return "unset " + strings.Join(EnvNames(), " ") + "\n" }

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
