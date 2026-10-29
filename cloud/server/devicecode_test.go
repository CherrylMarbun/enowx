package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
)

// send posts body as JSON with token, decoding a 200 into out; it returns
// the status and the error the server said.
func send(t *testing.T, method, url, token string, body, out any) (int, string) {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, bytes.NewReader(data))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if (res.StatusCode == http.StatusOK || res.StatusCode == http.StatusConflict) && out != nil { // a conflict answers with the item as it is
		json.NewDecoder(res.Body).Decode(out)
		return res.StatusCode, ""
	}
	var e api.Error
	json.NewDecoder(res.Body).Decode(&e)
	return res.StatusCode, e.Error
}

// A host signs in with a code its member approves from a device already
// signed in; the code works once, and not after it expires.
func TestHostSignsInWithACode(t *testing.T) {
	ctx := context.Background()
	srv, db := startWithDB(t, Config{URL: "https://cloud.example.com"})
	ada, _ := db.GitHubAccount(ctx, 101, "ada", "Ada", "", "")
	appKey, _ := seal.NewDeviceKey()
	_, macToken, _ := db.AddDevice(ctx, ada.ID, "Ada's Mac", api.DeviceApp, appKey.Public())

	hostKey, _ := seal.NewDeviceKey()
	var dc api.DeviceCode
	if code, msg := send(t, "POST", srv.URL+api.PathDeviceCode, "", api.DeviceCodeParams{Name: "vps", Kind: api.DeviceHost, PublicKey: hostKey.Public()}, &dc); code != http.StatusOK {
		t.Fatalf("code: %d %s", code, msg)
	}
	if !regexp.MustCompile(`^[B-Z]{4}-[B-Z]{4}$`).MatchString(dc.UserCode) || dc.Interval <= 0 {
		t.Fatalf("device code: %+v", dc)
	}
	token := func() (int, string, api.SignedIn) {
		var in api.SignedIn
		code, msg := send(t, "POST", srv.URL+api.PathDeviceToken, "", api.DeviceTokenParams{DeviceCode: dc.DeviceCode}, &in)
		return code, msg, in
	}
	if code, msg, _ := token(); code != http.StatusBadRequest || msg != api.ErrPending {
		t.Fatalf("before approval: %d %s", code, msg)
	}
	typed := "/v1/auth/device/pending/" + dc.UserCode[:4] + dc.UserCode[5:] // as typed: no dash
	if code, _ := send(t, "GET", srv.URL+typed, "", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("pending asked by no one: %d", code)
	}
	var pending api.PendingDevice
	if code, msg := send(t, "GET", srv.URL+typed, macToken, nil, &pending); code != http.StatusOK || pending.Name != "vps" || pending.Kind != api.DeviceHost {
		t.Fatalf("pending: %d %s %+v", code, msg, pending)
	}
	if code, msg := send(t, "POST", srv.URL+api.PathDeviceApprove, macToken, api.DeviceApproveParams{UserCode: dc.UserCode}, nil); code != http.StatusNoContent {
		t.Fatalf("approve: %d %s", code, msg)
	}
	code, msg, in := token()
	if code != http.StatusOK || in.Me.Account.ID != ada.ID || in.Me.Device.Kind != api.DeviceHost || !bytes.Equal(in.Me.Device.PublicKey, hostKey.Public()) {
		t.Fatalf("token: %d %s %+v", code, msg, in)
	}
	var me api.Me
	if call(t, srv.URL, "GET", api.PathMe, in.Token, &me) != http.StatusOK || me.Device.Name != "vps" {
		t.Fatalf("the host's token: %+v", me)
	}
	if code, msg, _ := token(); code != http.StatusBadRequest || msg != api.ErrExpired {
		t.Fatalf("the code again: %d %s", code, msg)
	}
	if code, _ := send(t, "POST", srv.URL+api.PathDeviceApprove, macToken, api.DeviceApproveParams{UserCode: dc.UserCode}, nil); code != http.StatusNotFound {
		t.Fatalf("approved again: %d", code)
	}

	// A code left too long expires.
	send(t, "POST", srv.URL+api.PathDeviceCode, "", api.DeviceCodeParams{Name: "late", Kind: api.DeviceHost, PublicKey: hostKey.Public()}, &dc)
	db.ExecContext(ctx, `UPDATE device_codes SET expires = '2000-01-01T00:00:00Z'`)
	if code, _ := send(t, "POST", srv.URL+api.PathDeviceApprove, macToken, api.DeviceApproveParams{UserCode: dc.UserCode}, nil); code != http.StatusNotFound {
		t.Fatalf("an expired code approved: %d", code)
	}
	if code, msg, _ := token(); code != http.StatusBadRequest || msg != api.ErrExpired {
		t.Fatalf("an expired code's token: %d %s", code, msg)
	}
	if code, _ := send(t, "POST", srv.URL+api.PathDeviceCode, "", api.DeviceCodeParams{Name: "x", Kind: "phone", PublicKey: hostKey.Public()}, nil); code != http.StatusBadRequest {
		t.Fatalf("a device of no known kind: %d", code)
	}
}
