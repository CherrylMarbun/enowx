package daemon

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/intercept"
	"github.com/enowx/enowx/core/vault"
	"github.com/enowx/enowx/proto"
)

// vaultApp is an app serving the vault: answer decides each request.
type vaultApp struct {
	*client.Client
	mu   sync.Mutex
	seen []proto.VaultRequest
}

func serveVault(t *testing.T, srv *Server, answer func(proto.VaultRequest) (proto.VaultLend, error)) *vaultApp {
	t.Helper()
	app := &vaultApp{}
	app.Client = connect(t, srv, func(ev proto.Event) {
		if ev.Name != proto.EventVaultRequest {
			return
		}
		var req proto.VaultRequest
		if json.Unmarshal(ev.Data, &req) != nil {
			return
		}
		app.mu.Lock()
		app.seen = append(app.seen, req)
		app.mu.Unlock()
		go func() {
			lend, err := answer(req)
			r := proto.VaultReply{ID: req.ID, Lend: lend}
			if err != nil {
				r.Error = err.Error()
			}
			_ = app.VaultReply(r)
		}()
	})
	if err := app.ServeVault(); err != nil {
		t.Fatal(err)
	}
	return app
}

// putVault gives the host a vault holding cs, as the app would.
func putVault(t *testing.T, cl *client.Client, cs ...vault.Credential) {
	t.Helper()
	f, key, err := vault.Create("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	v := &vault.Vault{}
	for _, c := range cs {
		if _, err := v.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Seal(v, key); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.VaultPut(proto.VaultCopy{Data: f.Bytes(), Index: v.Index(f.Updated)}); err != nil {
		t.Fatal(err)
	}
}

func waitFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && strings.HasSuffix(string(data), "\n") {
			return string(data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never written", path)
	return ""
}

// A host keeps the vault's ciphertext and index only; a session started
// with a credential gets its variables and files from the app, and git's
// helper; the values are masked in Traffic, and go with the session.
func TestSessionsGetCredentialsTheAppLends(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh")
	}
	srv := startServer(t)
	srv.HookBin = "/usr/local/bin/enowx-app"
	cl := connect(t, srv, nil)
	if _, err := cl.VaultPut(proto.VaultCopy{Data: []byte(`{"token":"plain"}`)}); err == nil {
		t.Fatal("a host took a file that isn't a sealed vault")
	}
	putVault(t, cl, vault.Credential{Platform: "github", Name: "gh", Fields: map[string]string{"token": "ghp_fakeTOKEN123456"}})
	idx, err := cl.VaultIndex()
	if err != nil || !idx.Present || len(idx.Credentials) != 1 || idx.Credentials[0].Hint != "…3456" {
		t.Fatalf("index: %+v %v", idx, err)
	}
	data, _ := os.ReadFile(filepath.Join(srv.vaultDir(), "vault.json"))
	if strings.Contains(string(data), "ghp_fake") {
		t.Fatal("the host's copy holds the token in the clear")
	}

	app := serveVault(t, srv, func(req proto.VaultRequest) (proto.VaultLend, error) {
		if req.Action != proto.VaultSession || len(req.Names) != 1 || req.Names[0] != "gh" {
			return proto.VaultLend{}, errors.New("unexpected " + req.Action)
		}
		return proto.VaultLend{Credentials: []string{"gh"}, Env: map[string]string{"GITHUB_TOKEN": "ghp_fakeTOKEN123456"},
			Files: map[string]string{"SA_FILE": "service account\n"}}, nil
	})
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	info, err := cl.CreateSession(proto.SessionCreateParams{Dir: dir, Cols: 80, Rows: 10, Creds: []string{"gh"},
		Command: []string{"sh", "-c", `printf '%s|%s|%s|' "$GITHUB_TOKEN" "$(cat "$SA_FILE")" "$GIT_CONFIG_VALUE_0" > out.txt; echo >> out.txt; sleep 30`}})
	if err != nil {
		t.Fatal(err)
	}
	got := waitFile(t, out)
	if !strings.HasPrefix(got, "ghp_fakeTOKEN123456|service account|!'/usr/local/bin/enowx-app' creds git-helper|") {
		t.Fatalf("the session's environment: %q", got)
	}
	if len(info.Creds) != 1 || info.Creds[0] != "gh" {
		t.Fatalf("info.Creds = %v", info.Creds)
	}
	if masked := intercept.MaskURL("https://example.com/?q=ghp_fakeTOKEN123456"); strings.Contains(masked, "fakeTOKEN") {
		t.Fatalf("Traffic shows a lent value: %s", masked)
	}
	run := filepath.Join(srv.vaultRunDir(), info.ID)
	if _, err := os.Stat(run); err != nil {
		t.Fatalf("no folder of the session's files: %v", err)
	}
	if err := cl.KillSession(info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(run); !os.IsNotExist(err) {
		t.Fatalf("its files stay after the session: %v", err)
	}
	if masked := intercept.MaskURL("https://example.com/?q=ghp_fakeTOKEN123456"); !strings.Contains(masked, "fakeTOKEN") {
		t.Fatal("still masked after the session went")
	}
	app.mu.Lock()
	if len(app.seen) != 1 || app.seen[0].Session != info.ID {
		t.Fatalf("requests: %+v", app.seen)
	}
	app.mu.Unlock()
}

// enowx creds asks the app through the daemon: what it lends comes back
// (files written for it), what it refuses says why, and the host records
// each use without values. With no app, it says to open it.
func TestCredentialRequestsAreAnsweredAndRecorded(t *testing.T) {
	srv := startServer(t)
	cl := connect(t, srv, nil)
	if _, err := cl.VaultAsk(proto.VaultAsk{Action: proto.VaultEnv, Name: "gh"}); err == nil || !strings.Contains(err.Error(), "open the app") {
		t.Fatalf("with no app: %v", err)
	}
	serveVault(t, srv, func(req proto.VaultRequest) (proto.VaultLend, error) {
		if req.Name == "prod" {
			return proto.VaultLend{}, errors.New("the user said no")
		}
		return proto.VaultLend{Credentials: []string{req.Name}, Env: map[string]string{"TOKEN": "tok_secret_value"},
			Files: map[string]string{"KEY_FILE": "-----KEY-----"}}, nil
	})
	lend, err := cl.VaultAsk(proto.VaultAsk{Action: proto.VaultEnv, Name: "gcp", Command: "gcloud projects list"})
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(lend.Env["KEY_FILE"]); err != nil || string(data) != "-----KEY-----" || lend.Files["KEY_FILE"] != lend.Env["KEY_FILE"] {
		t.Fatalf("the file: %q %v %+v", data, err, lend)
	}
	if _, err := cl.VaultAsk(proto.VaultAsk{Action: proto.VaultEnv, Name: "prod"}); err == nil || !strings.Contains(err.Error(), "said no") {
		t.Fatalf("refused: %v", err)
	}
	audit, err := cl.VaultAudit(0)
	if err != nil || len(audit) != 3 {
		t.Fatalf("audit: %+v %v", audit, err)
	}
	if !audit[1].Allowed || audit[1].Credential != "gcp" || audit[1].Detail != "gcloud projects list" || audit[2].Allowed {
		t.Fatalf("audit: %+v", audit)
	}
	raw, _ := json.Marshal(audit)
	if strings.Contains(string(raw), "tok_secret") {
		t.Fatal("the audit holds a value")
	}
}

// A session gets an SSH agent of the vault's keys: it lists their public
// keys from the host's index, and the app makes each signature.
func TestSessionSSHAgentSignsThroughTheApp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no agent on Windows")
	}
	srv := startServer(t)
	srv.HookBin = "/usr/local/bin/enowx-app"
	cl := connect(t, srv, nil)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, _ := ssh.MarshalPrivateKey(priv, "")
	private := string(pem.EncodeToMemory(block))
	pub, _ := vault.PublicKey(private)
	putVault(t, cl, vault.Credential{Platform: "ssh", Name: "deploy-key", Fields: map[string]string{"private_key": private, "public_key": pub}})
	serveVault(t, srv, func(req proto.VaultRequest) (proto.VaultLend, error) {
		if req.Action != proto.VaultSign || !vault.KeyMatches(private, req.KeyBlob) {
			return proto.VaultLend{}, errors.New("unexpected")
		}
		sig, err := vault.SignSSH(private, req.Data, req.Flags)
		if err != nil {
			return proto.VaultLend{}, err
		}
		return proto.VaultLend{Signature: sig.Blob, SigFormat: sig.Format}, nil
	})
	dir := t.TempDir()
	if _, err := cl.CreateSession(proto.SessionCreateParams{Dir: dir, Cols: 80, Rows: 10,
		Command: []string{"sh", "-c", `echo "$SSH_AUTH_SOCK" > out.txt; sleep 30`}}); err != nil {
		t.Fatal(err)
	}
	sock := strings.TrimSpace(waitFile(t, filepath.Join(dir, "out.txt")))
	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ag := agent.NewClient(conn)
	keys, err := ag.List()
	if err != nil || len(keys) != 1 || keys[0].Comment != "deploy-key" {
		t.Fatalf("keys: %v %v", keys, err)
	}
	sig, err := ag.Sign(keys[0], []byte("session id"))
	if err != nil {
		t.Fatal(err)
	}
	pk, _ := ssh.ParsePublicKey(keys[0].Blob)
	if err := pk.Verify([]byte("session id"), sig); err != nil {
		t.Fatal(err)
	}
}
