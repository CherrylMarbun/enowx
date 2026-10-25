package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/enowx/enowx/core/vault"
)

// GitHub signed in in the browser: the user gives their OAuth app's client
// ID, sees the code (the page opened for them), and once they agree there
// the token fills in, to add; the client ID is kept for next time.
func TestVaultSignInInTheBrowser(t *testing.T) {
	agreed := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/code":
			json.NewEncoder(w).Encode(map[string]any{"device_code": "d1", "user_code": "WXYZ-9876", "verification_uri": "https://example.com/login/device", "expires_in": 600, "interval": 1})
		case "/token":
			if !agreed {
				agreed = true
				json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"access_token": "gho_browserTOKEN4321"})
		}
	}))
	t.Cleanup(srv.Close)
	i := slices.IndexFunc(vault.Platforms, func(p vault.Platform) bool { return p.ID == "github" })
	old := vault.Platforms[i].Device
	d := *old
	d.CodeURL, d.TokenURL = srv.URL+"/code", srv.URL+"/token"
	vault.Platforms[i].Device = &d
	t.Cleanup(func() { vault.Platforms[i].Device = old })
	var opened []string
	oldOpen := openExternal
	openExternal = func(u string) error { opened = append(opened, u); return nil }
	t.Cleanup(func() { openExternal = oldOpen })

	h := newVaultHarness(t)
	if err := h.app.vaultCreate("correct horse", false); err != nil {
		t.Fatal(err)
	}
	h.app.vaultOf().view.OpenAdd("github")
	h.settle("the form", func() bool { return h.tt.HasText("Or sign in in the browser") })
	h.click("Your OAuth app's client ID")
	h.tt.Type("Iv1.mine")
	h.click("Sign in")
	h.settle("the code", func() bool { return h.tt.HasText("WXYZ-9876") })
	shot(t, h.tt, "vault-device.png")
	h.settle("signed in", func() bool { return h.tt.HasText("Signed in: the token is filled in. Add it to keep it.") })
	if len(opened) != 1 || opened[0] != "https://example.com/login/device" {
		t.Fatalf("opened %v", opened)
	}
	h.click("Add")
	h.settle("the credential listed", func() bool { return h.tt.HasText("…4321") })
	if h.app.state.OAuthClients["github"] != "Iv1.mine" {
		t.Fatalf("client IDs kept: %v", h.app.state.OAuthClients)
	}
}
