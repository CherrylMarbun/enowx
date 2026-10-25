package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/proto"
)

// git and docker ask enowX for logins as their credential helpers: a
// session's environment names enowx creds git-helper to git, and
// docker-credential-enowx (a link to this program) to docker.

// gitHelper answers git's credential helper protocol: "get" prints the
// vault's login for the server git asks about; store and erase do
// nothing, the vault being the app's to change. With no login, it prints
// nothing, and git goes on to its next helper or asks.
func (r runner) gitHelper(session string, args []string, in io.Reader) error {
	if len(args) == 0 || args[0] != "get" {
		_, _ = io.Copy(io.Discard, in)
		return nil
	}
	attrs := map[string]string{}
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), "="); ok {
			attrs[k] = v
		}
	}
	host := attrs["host"]
	if host == "" || (attrs["protocol"] != "" && attrs["protocol"] != "https" && attrs["protocol"] != "http") {
		return nil
	}
	lend, err := r.c.VaultAsk(proto.VaultAsk{Action: proto.VaultGit, Host: host, Session: session, Command: "git " + attrs["protocol"] + "://" + host + "/" + attrs["path"]})
	if err != nil {
		fmt.Fprintln(os.Stderr, "enowX vault:", err)
		return nil
	}
	if lend.Secret != "" {
		fmt.Fprintf(r.out, "username=%s\npassword=%s\n", lend.Username, lend.Secret)
	}
	return nil
}

// IsDockerHelper reports whether the program runs as docker's credential
// helper, by the name it was started as.
func IsDockerHelper(argv0 string) bool {
	return strings.HasPrefix(strings.TrimSuffix(filepath.Base(argv0), ".exe"), "docker-credential-")
}

// DockerHelper answers docker's credential helper protocol: get prints
// the vault's login for a registry; list is empty, store and erase are
// refused, the vault being the app's to change.
func DockerHelper(args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: docker-credential-enowx get|list|store|erase")
	}
	switch args[0] {
	case "list":
		_, err := io.WriteString(out, "{}\n")
		return err
	case "store", "erase":
		_, _ = io.Copy(io.Discard, in)
		return fmt.Errorf("the enowX vault keeps its logins in the app: change them there")
	case "get":
	default:
		return fmt.Errorf("unknown action %q", args[0])
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	server := strings.TrimSpace(string(data))
	c, err := Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	lend, err := c.VaultAsk(proto.VaultAsk{Action: proto.VaultDocker, Host: server, Session: os.Getenv(EnvSession), Command: "docker " + server})
	if err != nil || lend.Secret == "" {
		// docker's word for none, so it goes on without a login.
		fmt.Fprintln(out, "credentials not found in native keychain")
		if err != nil {
			fmt.Fprintln(os.Stderr, "enowX vault:", err)
		}
		return fmt.Errorf("no login for %s", server)
	}
	return json.NewEncoder(out).Encode(map[string]string{"ServerURL": server, "Username": lend.Username, "Secret": lend.Secret})
}
