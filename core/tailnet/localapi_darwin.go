package tailnet

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// localStatus asks the Tailscale app for its status over its local API. The
// CLI inside Tailscale.app tries to start the app's GUI when a program
// other than a terminal runs it, and fails, so on macOS this comes first.
func localStatus(ctx context.Context) ([]byte, error) {
	port, token, err := localAPICredentials()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://local-tailscaled.sock/localapi/v0/status", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("", token)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		},
	}}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tailnet: local API: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("tailnet: local API: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tailnet: local API: %s: %s", res.Status, firstLine(strings.TrimSpace(string(body))))
	}
	return body, nil
}

var errNoLocalAPI = errors.New("tailnet: no Tailscale app running")

// localAPICredentials finds the port and token the Tailscale app leaves for
// programs of the same user: an open file of the App Store app, else the
// files the standalone app writes to /Library/Tailscale.
func localAPICredentials() (int, string, error) {
	out, err := exec.Command("lsof", "-n", "-a", fmt.Sprintf("-u%d", os.Getuid()), "-c", "IPNExtension", "-F").Output()
	if err == nil {
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			_, after, ok := bytes.Cut(sc.Bytes(), []byte(".tailscale.ipn.macos/sameuserproof-"))
			if !ok {
				continue
			}
			portStr, token, ok := strings.Cut(string(after), "-")
			if port, err := strconv.Atoi(portStr); ok && err == nil && token != "" {
				return port, token, nil
			}
		}
	}
	const shared = "/Library/Tailscale"
	portStr, err := os.Readlink(filepath.Join(shared, "ipnport"))
	if err != nil {
		return 0, "", errNoLocalAPI
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, "", errNoLocalAPI
	}
	token, err := os.ReadFile(filepath.Join(shared, "sameuserproof-"+portStr))
	if err != nil || len(bytes.TrimSpace(token)) == 0 {
		return 0, "", errNoLocalAPI
	}
	return port, string(bytes.TrimSpace(token)), nil
}
