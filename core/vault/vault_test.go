package vault

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestSealedVaultOpensWithItsPasswordOrKeyOnly(t *testing.T) {
	f, key, err := Create("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	v := &Vault{}
	if _, err := v.Add(Credential{Platform: "github", Account: "ada", Fields: map[string]string{"token": "ghp_fake_0123456789abcd"}}); err != nil {
		t.Fatal(err)
	}
	if err := f.Seal(v, key); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(f.Bytes(), []byte("ghp_fake")) {
		t.Fatal("the file holds the token in the clear")
	}
	g, err := Parse(f.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Unlock("wrong horse"); err != ErrWrongPassword {
		t.Fatalf("wrong password: %v", err)
	}
	k2, err := g.Unlock("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.Open(k2)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := got.Find("github-ada")
	if !ok || c.Fields["token"] != "ghp_fake_0123456789abcd" || !c.Default {
		t.Fatalf("got %+v", got)
	}
	if _, err := g.Open(make([]byte, 32)); err != ErrWrongKey {
		t.Fatalf("another key: %v", err)
	}
	if err := g.ChangePassword("correct horse", "battery staple"); err != nil {
		t.Fatal(err)
	}
	if k3, err := g.Unlock("battery staple"); err != nil || !bytes.Equal(k3, key) {
		t.Fatalf("after the change: %v", err)
	}
	if _, _, err := Create("short"); err == nil {
		t.Fatal("a short master password")
	}
}

func TestIndexHasNoValues(t *testing.T) {
	v := &Vault{}
	v.Add(Credential{Platform: "aws", Name: "aws-prod", Fields: map[string]string{"access_key_id": "AKIAFAKE", "secret_access_key": "fakeSecretKeyValue1234"}})
	info := v.Index(time.Now()).Credentials[0]
	raw := info.Hint + " " + strings.Join(info.Env, ",")
	if strings.Contains(raw, "fakeSecret") || info.Hint != "…1234" {
		t.Fatalf("index: %+v", info)
	}
	if !strings.Contains(raw, "AWS_SECRET_ACCESS_KEY") {
		t.Fatalf("env names: %v", info.Env)
	}
	if _, err := v.Add(Credential{Platform: "aws", Name: "aws prod", Fields: map[string]string{"x": "y"}}); err == nil {
		t.Fatal("a name with a space")
	}
}

func TestLendGivesVariablesAndFiles(t *testing.T) {
	c := Credential{Platform: "pypi", Fields: map[string]string{"token": "pypi-faketoken"}}
	env, _ := c.Lend()
	if env["TWINE_USERNAME"] != "__token__" || env["TWINE_PASSWORD"] != "pypi-faketoken" {
		t.Fatalf("pypi: %v", env)
	}
	g := Credential{Platform: "gcp", Fields: map[string]string{"json": `{"type":"service_account"}`, "project": "p1"}}
	env, files := g.Lend()
	if files["GOOGLE_APPLICATION_CREDENTIALS"] == "" || env["GOOGLE_CLOUD_PROJECT"] != "p1" || env["GOOGLE_APPLICATION_CREDENTIALS"] != "" {
		t.Fatalf("gcp: %v %v", env, files)
	}
	e := Credential{Type: TypeEnvFile, Platform: "env-file", Fields: map[string]string{"env": "# x\nexport A=1\nB=\"two words\"\n"}}
	env, _ = e.Lend()
	if env["A"] != "1" || env["B"] != "two words" {
		t.Fatalf("env file: %v", env)
	}
	if s := e.Secrets(); len(s) != 2 {
		t.Fatalf("env file secrets: %v", s)
	}
}

func TestGitAndDockerLogins(t *testing.T) {
	gh := Credential{Platform: "github", Account: "ada", Fields: map[string]string{"token": "ghp_x"}}
	if u, s, ok := gh.GitLogin("github.com"); !ok || u != "x-access-token" || s != "ghp_x" {
		t.Fatalf("git: %s %s %v", u, s, ok)
	}
	if _, _, ok := gh.GitLogin("gitlab.com"); ok {
		t.Fatal("github for gitlab")
	}
	if u, _, ok := gh.DockerLogin("ghcr.io"); !ok || u != "ada" {
		t.Fatalf("ghcr: %s %v", u, ok)
	}
	hub := Credential{Platform: "dockerhub", Fields: map[string]string{"username": "ada", "password": "dckr_x"}}
	if u, s, ok := hub.DockerLogin("https://index.docker.io/v1/"); !ok || u != "ada" || s != "dckr_x" {
		t.Fatalf("hub: %s %s %v", u, s, ok)
	}
	tea := Credential{Platform: "gitea", Fields: map[string]string{"token": "t", "url": "https://git.example.com/"}}
	if _, _, ok := tea.GitLogin("git.example.com"); !ok {
		t.Fatal("gitea's own server")
	}
}

func TestPolicy(t *testing.T) {
	now := time.Now()
	c := Credential{Name: "aws-prod", Hosts: []string{"vps1"}}
	if d, _ := c.Allowed("vps1", "", now); d != Allow {
		t.Fatal("allowed host")
	}
	if d, _ := c.Allowed("mac", "", now); d != Ask {
		t.Fatal("another host asks")
	}
	c.HostsOnly = true
	if d, why := c.Allowed("mac", "", now); d != Deny || why == "" {
		t.Fatal("hosts only")
	}
	c.Projects = []string{"api"}
	if d, _ := c.Allowed("vps1", "/srv/web", now); d != Deny {
		t.Fatal("another project")
	}
	if d, _ := c.Allowed("vps1", "/srv/api", now); d != Allow {
		t.Fatal("its project, by name")
	}
	c.Expires = now.Add(-time.Hour)
	if d, why := c.Allowed("vps1", "/srv/api", now); d != Deny || !strings.Contains(why, "expired") {
		t.Fatal("expired")
	}
	c.Expires, c.Revoked = time.Time{}, true
	if d, why := c.Allowed("vps1", "/srv/api", now); d != Deny || !strings.Contains(why, "revoked") {
		t.Fatal("revoked")
	}
	soon := Credential{Expires: now.Add(48 * time.Hour)}
	if !soon.ExpiresSoon(now, 7*24*time.Hour) {
		t.Fatal("expires soon")
	}
}

func TestPickPrefersTheProjectsThenTheDefault(t *testing.T) {
	v := &Vault{}
	v.Add(Credential{Platform: "github", Name: "gh-me", Fields: map[string]string{"token": "a"}})
	v.Add(Credential{Platform: "github", Name: "gh-work", Projects: []string{"/srv/work"}, Fields: map[string]string{"token": "b"}})
	if got := v.Pick("", "github", "/srv/work", nil); got[0].Name != "gh-work" {
		t.Fatalf("work project: %s", got[0].Name)
	}
	if got := v.Pick("", "github", "/srv/home", nil); got[0].Name != "gh-me" {
		t.Fatalf("elsewhere: %s", got[0].Name)
	}
	if err := v.SetDefault("gh-work"); err != nil {
		t.Fatal(err)
	}
	if err := v.Remove("gh-work"); err != nil {
		t.Fatal(err)
	}
	if c, _ := v.Find("gh-me"); !c.Default {
		t.Fatal("the default passes on")
	}
}

func TestRedactorMasksSecretsSplitAcrossWrites(t *testing.T) {
	var b bytes.Buffer
	r := NewRedactor(&b, []string{"supersecret123", "tiny"})
	for _, part := range []string{"token=super", "secret123 and ", base64.StdEncoding.EncodeToString([]byte("supersecret123")), " tiny"} {
		r.Write([]byte(part))
	}
	r.Close()
	got := b.String()
	if strings.Contains(got, "supersecret") || strings.Contains(got, "c3VwZXJzZWNyZXQxMjM=") || !strings.Contains(got, "•••t123") || !strings.Contains(got, "tiny") {
		t.Fatalf("got %q", got)
	}
}

func TestFillEscapesJSON(t *testing.T) {
	got := fill(`{"username":"{username}","password":"{password}"}`, map[string]string{"username": "ada", "password": `p"w`}, true)
	if got != `{"username":"ada","password":"p\"w"}` {
		t.Fatalf("got %s", got)
	}
	if got := fill("{url|https://gitlab.com}/api", map[string]string{"url": "https://x.org/"}, false); got != "https://x.org/api" {
		t.Fatalf("got %s", got)
	}
	if got := fill("{url|https://gitlab.com}/api", nil, false); got != "https://gitlab.com/api" {
		t.Fatalf("got %s", got)
	}
}
