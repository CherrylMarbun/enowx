package vault

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/enowx/enowx/proto"
)

// Test asks the credential's platform whether it still works, only when
// the user (or an agent through the app) asks: a status, and why. It
// never puts a value in what it returns.
func Test(ctx context.Context, client *http.Client, c Credential) (status, message string) {
	if st := c.State(time.Now()); st == proto.CredRevoked || st == proto.CredExpired {
		return st, "not checked: " + st
	}
	if c.Type == TypeSSHKey {
		return testSSHKey(c)
	}
	p := Lookup(c.Platform)
	if p.Check == nil {
		return proto.CredUnchecked, p.Name + " has no check: it shows whether it works when used"
	}
	req, err := checkRequest(ctx, p.Check, c.Fields)
	if err != nil {
		return proto.CredUnchecked, err.Error()
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return proto.CredUnchecked, "couldn't reach " + req.URL.Host + ": " + scrub(err.Error(), c.Secrets())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return proto.CredInvalid, fmt.Sprintf("%s refused it (%s)", p.Name, resp.Status)
	case resp.StatusCode/100 != 2:
		return proto.CredUnchecked, fmt.Sprintf("%s answered %s", p.Name, resp.Status)
	case p.Check.Fail != "" && strings.Contains(string(body), p.Check.Fail):
		return proto.CredInvalid, p.Name + " refused it"
	}
	return proto.CredValid, p.Name + " accepts it"
}

// checkRequest fills a check's templates with the credential's values.
func checkRequest(ctx context.Context, ch *Check, fields map[string]string) (*http.Request, error) {
	method := ch.Method
	if method == "" {
		method = http.MethodGet
	}
	u := fill(ch.URL, fields, false)
	if strings.Contains(u, "{") || !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return nil, fmt.Errorf("fill in its server URL to check it")
	}
	var body io.Reader
	if ch.Body != "" {
		json := strings.HasPrefix(strings.TrimSpace(ch.Body), "{")
		body = strings.NewReader(fill(ch.Body, fields, json))
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("its server URL isn't one")
	}
	req.Header.Set("User-Agent", "enowX")
	for k, v := range ch.Header {
		req.Header.Set(k, fill(v, fields, false))
	}
	if ch.Basic[0] != "" {
		req.SetBasicAuth(fill(ch.Basic[0], fields, false), fill(ch.Basic[1], fields, false))
	}
	return req, nil
}

// fill replaces "{key}" and "{key|default}" with field values, escaped
// for a JSON string when json.
func fill(tmpl string, fields map[string]string, json bool) string {
	var b strings.Builder
	for {
		i := strings.Index(tmpl, "{")
		if i < 0 {
			b.WriteString(tmpl)
			return b.String()
		}
		j := strings.Index(tmpl[i:], "}")
		if j < 0 {
			b.WriteString(tmpl)
			return b.String()
		}
		if strings.ContainsAny(tmpl[i+1:i+j], `"{ `) {
			// A JSON object's brace, not a placeholder.
			b.WriteString(tmpl[:i+1])
			tmpl = tmpl[i+1:]
			continue
		}
		b.WriteString(tmpl[:i])
		key, def, _ := strings.Cut(tmpl[i+1:i+j], "|")
		v := fields[key]
		if v == "" {
			v = def
		}
		if json {
			v = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
		}
		if key == "url" {
			v = strings.TrimRight(v, "/")
		}
		b.WriteString(v)
		tmpl = tmpl[i+j+1:]
	}
}

// testSSHKey checks that the private key reads and matches its public
// key, filling the public key in when missing.
func testSSHKey(c Credential) (string, string) {
	signer, err := ssh.ParsePrivateKey([]byte(c.Fields["private_key"]))
	if err != nil {
		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			return proto.CredInvalid, "the key has a passphrase: add it decrypted (ssh-keygen -p) to the vault"
		}
		return proto.CredInvalid, "not a private key enowX reads"
	}
	if pub := strings.TrimSpace(c.Fields["public_key"]); pub != "" {
		want, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pub))
		if err != nil || string(want.Marshal()) != string(signer.PublicKey().Marshal()) {
			return proto.CredInvalid, "the public key isn't the private key's"
		}
	}
	return proto.CredValid, "a " + signer.PublicKey().Type() + " key"
}

// PublicKey is an SSH private key's public key, as in authorized_keys.
func PublicKey(private string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(private))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), nil
}

// scrub masks secrets in s, as an error may quote a URL with a token.
func scrub(s string, secrets []string) string {
	for _, v := range secrets {
		if len(v) >= 6 {
			s = strings.ReplaceAll(s, v, Mask(v))
		}
	}
	return s
}
