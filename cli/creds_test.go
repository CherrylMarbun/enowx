package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/vault"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// An agent uses the user's credentials by name: it lists them without
// values, runs a command with one (what it prints masked), and git gets a
// login through the helper; whatever the app refuses says why.
func TestCredsFromTheCommandLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh")
	}
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	dial := func(onEvent func(proto.Event)) *client.Client {
		a, b := net.Pipe()
		go srv.ServeConn(a)
		c, err := client.New(b, onEvent)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}
	c := dial(nil)
	var out bytes.Buffer
	run := func(args ...string) (string, error) {
		out.Reset()
		err := (runner{c: c, out: &out}).creds(args)
		return out.String(), err
	}
	if got, _ := run(); !strings.Contains(got, "No vault on this host yet") {
		t.Fatalf("no vault: %q", got)
	}

	// The app: a vault with a GitHub token, lent to whoever asks but prod.
	f, key, _ := vault.Create("correct horse")
	v := &vault.Vault{}
	v.Add(vault.Credential{Platform: "github", Name: "gh", Account: "ada", Fields: map[string]string{"token": "ghp_fakeTOKEN123456"}})
	v.Add(vault.Credential{Platform: "aws", Name: "aws-prod", Fields: map[string]string{"access_key_id": "AKIAFAKE", "secret_access_key": "fakeSecretKey987654"}})
	f.Seal(v, key)
	if _, err := c.VaultPut(proto.VaultCopy{Data: f.Bytes(), Index: v.Index(f.Updated)}); err != nil {
		t.Fatal(err)
	}
	var app *client.Client
	app = dial(func(ev proto.Event) {
		if ev.Name != proto.EventVaultRequest {
			return
		}
		var req proto.VaultRequest
		json.Unmarshal(ev.Data, &req)
		go func() {
			reply := proto.VaultReply{ID: req.ID}
			switch {
			case req.Name == "aws-prod":
				reply.Error = "aws-prod may not be used on this host"
			case req.Action == proto.VaultEnv:
				cred, _ := v.Find(req.Name)
				env, _ := cred.Lend()
				reply.Lend = proto.VaultLend{Credentials: []string{cred.Name}, Env: env}
			case req.Action == proto.VaultGit:
				cred, _ := v.Find("gh")
				u, s, _ := cred.GitLogin(req.Host)
				reply.Lend = proto.VaultLend{Credentials: []string{"gh"}, Username: u, Secret: s}
			default:
				reply.Error = "unexpected " + req.Action
			}
			app.VaultReply(reply)
		}()
	})
	if err := app.ServeVault(); err != nil {
		t.Fatal(err)
	}

	list, err := run("list")
	if err != nil || !strings.Contains(list, "gh *") || !strings.Contains(list, "GH_TOKEN GITHUB_TOKEN (git: github.com)") || strings.Contains(list, "fakeTOKEN") {
		t.Fatalf("list: %v\n%s", err, list)
	}
	got, err := run("use", "gh", "--", "sh", "-c", `echo "token is $GITHUB_TOKEN"`)
	if err != nil || strings.Contains(got, "fakeTOKEN") || !strings.Contains(got, "token is •••3456") {
		t.Fatalf("use: %v %q", err, got)
	}
	if _, err := run("use", "gh", "--", "sh", "-c", "exit 3"); err == nil || !strings.Contains(err.Error(), "status 3") {
		t.Fatalf("a failing command: %v", err)
	}
	if _, err := run("use", "aws-prod", "--", "true"); err == nil || !strings.Contains(err.Error(), "may not be used") {
		t.Fatalf("refused: %v", err)
	}
	if _, err := run("use", "--", "true"); err == nil || !strings.Contains(err.Error(), "no credentials") {
		t.Fatalf("no session's credentials: %v", err)
	}

	out.Reset()
	if err := (runner{c: c, out: &out}).gitHelper("", []string{"get"}, strings.NewReader("protocol=https\nhost=github.com\npath=ada/repo.git\n")); err != nil {
		t.Fatal(err)
	}
	if out.String() != "username=x-access-token\npassword=ghp_fakeTOKEN123456\n" {
		t.Fatalf("git helper: %q", out.String())
	}
	audit, _ := run("audit")
	if !strings.Contains(audit, "git: git https://github.com/ada/repo.git") || !strings.Contains(audit, "refused") {
		t.Fatalf("audit:\n%s", audit)
	}
	if !IsDockerHelper("/tmp/x/docker-credential-enowx") || IsDockerHelper("enowx-app") {
		t.Fatal("docker helper's name")
	}
	if err := DockerHelper([]string{"store"}, strings.NewReader("{}"), &out); err == nil {
		t.Fatal("docker stored a login in the vault")
	}
}
