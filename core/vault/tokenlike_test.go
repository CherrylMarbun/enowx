package vault

import "testing"

func TestTokenLike(t *testing.T) {
	for _, c := range []struct {
		id, s string
		want  bool
	}{
		{"github", "ghp_" + "abcdefghijklmnopqrstuvwxyzABCDEF0123", true},
		{"github", "github_pat_11ABCDEFG0abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOP", true},
		{"github", "sk-ant-api03-abcdefghijklmnopqrstuvwxyz0123456789", false},
		{"anthropic", "sk-ant-api03-abcdefghijklmnopqrstuvwxyz0123456789", true},
		{"notion", "ntn_" + "123456789012345678901234567890abcd", true},
		{"cloudflare", "Zz9vQm0p-ExAmPlEtOkEn_1234567890abcdef", true}, // no known shape: a long word
		{"cloudflare", "https://dash.cloudflare.com/profile/api-tokens", false},
		{"cloudflare", "hello world, this is a sentence", false},
		{"cloudflare", "short", false},
	} {
		if got := TokenLike(c.id, c.s); got != c.want {
			t.Errorf("TokenLike(%q, %q) = %v", c.id, c.s, got)
		}
	}
	if TokenField(Lookup("aws")) != "secret_access_key" || TokenField(Lookup("github")) != "token" {
		t.Error("the field a copied token goes in")
	}
}
