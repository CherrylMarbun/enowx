package vault

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/enowx/enowx/proto"
)

func TestScanFindsToolsLogins(t *testing.T) {
	home := t.TempDir()
	write := func(rel, data string) {
		p := filepath.Join(home, rel)
		os.MkdirAll(filepath.Dir(p), 0o700)
		os.WriteFile(p, []byte(data), 0o600)
	}
	write(".config/gh/hosts.yml", "github.com:\n    user: ada\n    git_protocol: https\n")
	write(".aws/credentials", "[default]\naws_access_key_id = AKIAFAKE\naws_secret_access_key = fakesecret\n[prod]\naws_access_key_id=AKIA2\naws_secret_access_key=s2\n")
	write(".aws/config", "[profile prod]\nregion = eu-central-1\n")
	auth := base64.StdEncoding.EncodeToString([]byte("ada:dckr_fake"))
	write(".docker/config.json", `{"auths":{"https://index.docker.io/v1/":{"auth":"`+auth+`"},"ghcr.io":{"auth":"`+auth+`"}}}`)
	write(".npmrc", "//registry.npmjs.org/:_authToken=npm_fake\n")
	write(".git-credentials", "https://ada:glpat-fake@gitlab.com\n")
	write(".wrangler/config/default.toml", "oauth_token = \"cf_fake\"\n")
	write(".supabase/access-token", "sbp_fake\n")
	write(".railway/config.json", `{"user":{"token":"rw_fake"}}`)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, _ := ssh.MarshalPrivateKey(priv, "ada@test")
	write(".ssh/id_ed25519", string(pem.EncodeToMemory(block)))
	write(".ssh/id_ed25519.pub", "ignored")

	ran := ""
	found := Scan(home, func(name string, args ...string) (string, error) {
		ran = name + " " + strings.Join(args, " ")
		return "gho_fromkeychain\n", nil
	})
	by := map[string]proto.VaultFound{}
	for _, f := range found {
		by[f.Platform+":"+f.Account] = f
	}
	if by["github:ada"].Fields["token"] != "gho_fromkeychain" || !strings.Contains(ran, "gh auth token") {
		t.Errorf("gh: %+v (%s)", by["github:ada"], ran)
	}
	if by["aws:prod"].Fields["region"] != "eu-central-1" || by["aws:default"].Fields["secret_access_key"] != "fakesecret" {
		t.Errorf("aws: %+v", found)
	}
	if by["dockerhub:ada"].Fields["password"] != "dckr_fake" || by["ghcr:ada"].Fields["username"] != "ada" {
		t.Errorf("docker: %+v", found)
	}
	if by["npm:"].Fields["token"] != "npm_fake" || by["gitlab:ada"].Fields["token"] != "glpat-fake" {
		t.Errorf("npm, git: %+v", found)
	}
	if by["cloudflare:"].Note == "" || by["supabase:"].Fields["token"] != "sbp_fake" || by["railway:"].Fields["token"] != "rw_fake" {
		t.Errorf("wrangler, supabase, railway: %+v", found)
	}
	if k := by["ssh:id_ed25519"]; !strings.HasPrefix(k.Fields["public_key"], "ssh-ed25519 ") || k.Name != "ssh-ed25519" {
		t.Errorf("ssh: %+v", k)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCheckAsksThePlatform(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token-123" {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	target, _ := url.Parse(srv.URL)
	client := srv.Client()
	base := client.Transport
	client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return base.RoundTrip(r)
	})
	good := Credential{Platform: "github", Fields: map[string]string{"token": "good-token-123"}}
	if st, msg := Test(context.Background(), client, good); st != proto.CredValid {
		t.Fatalf("good: %s %s", st, msg)
	}
	bad := Credential{Platform: "github", Fields: map[string]string{"token": "bad-token-xyz"}}
	if st, msg := Test(context.Background(), client, bad); st != proto.CredInvalid || strings.Contains(msg, "bad-token") {
		t.Fatalf("bad: %s %s", st, msg)
	}
	if st, _ := Test(context.Background(), client, Credential{Platform: "fly", Fields: map[string]string{"token": "x"}}); st != proto.CredUnchecked {
		t.Fatal("no check")
	}
	revoked := good
	revoked.Revoked = true
	if st, _ := Test(context.Background(), client, revoked); st != proto.CredRevoked {
		t.Fatal("revoked")
	}
}

func TestSSHAgentSignsWithTheAppsKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, _ := ssh.MarshalPrivateKey(priv, "")
	private := string(pem.EncodeToMemory(block))
	pub, err := PublicKey(private)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := testSSHKey(Credential{Fields: map[string]string{"private_key": private, "public_key": pub}}); st != proto.CredValid {
		t.Fatal("the key checks")
	}
	a := &SSHAgent{
		Keys: func() []string { return []string{pub + " ada@vault"} },
		SignData: func(key, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
			if !KeyMatches(private, key) {
				t.Error("asked for another key")
			}
			return SignSSH(private, data, uint32(flags))
		},
	}
	c1, c2 := net.Pipe()
	go a.Serve(c1)
	client := agent.NewClient(c2)
	keys, err := client.List()
	if err != nil || len(keys) != 1 || keys[0].Comment != "ada@vault" {
		t.Fatalf("list: %v %v", keys, err)
	}
	sig, err := client.Sign(keys[0], []byte("challenge"))
	if err != nil {
		t.Fatal(err)
	}
	pk, _ := ssh.ParsePublicKey(keys[0].Blob)
	if err := pk.Verify([]byte("challenge"), sig); err != nil {
		t.Fatal(err)
	}
	if err := client.Add(agent.AddedKey{PrivateKey: priv}); err == nil {
		t.Fatal("adding a key to the vault's agent")
	}
	c2.Close()
}

func TestAuditKeepsUses(t *testing.T) {
	a := OpenAudit(filepath.Join(t.TempDir(), "vault", "audit.jsonl"))
	for i := range 3 {
		a.Add(proto.VaultAuditEntry{Time: time.Now(), Action: "env", Credential: "gh", Allowed: i != 1})
	}
	if got := a.List(2); len(got) != 2 || !got[1].Allowed || got[0].Allowed {
		t.Fatalf("got %+v", got)
	}
}
