package vault

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// A platform's device flow: a code to type, waits while the user hasn't
// agreed (slower when told), then the token; a declined login says so.
func TestDeviceFlow(t *testing.T) {
	var mu sync.Mutex
	polls, deny := 0, false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("client_id") != "Iv1.test" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized_client"})
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/code":
			json.NewEncoder(w).Encode(map[string]any{"device_code": "dev-1", "user_code": "ABCD-1234",
				"verification_uri": "https://example.com/device", "expires_in": 60, "interval": 0})
		case "/token":
			if r.Form.Get("device_code") != "dev-1" || r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request"})
				return
			}
			polls++
			switch {
			case deny:
				json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
			case polls == 1:
				json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			case polls == 2:
				json.NewEncoder(w).Encode(map[string]any{"error": "slow_down", "interval": 1})
			default:
				json.NewEncoder(w).Encode(map[string]string{"access_token": "gho_fromthedevice", "token_type": "bearer"})
			}
		}
	}))
	defer srv.Close()
	d := Device{CodeURL: srv.URL + "/code", TokenURL: srv.URL + "/token", Field: "token"}
	ctx := context.Background()
	if _, err := StartDevice(ctx, d, ""); err == nil {
		t.Fatal("started without a client ID")
	}
	if _, err := StartDevice(ctx, d, "someone-else"); err == nil {
		t.Fatal("a refused client ID gave a code")
	}
	dc, err := StartDevice(ctx, d, "Iv1.test")
	if err != nil {
		t.Fatal(err)
	}
	if dc.UserCode != "ABCD-1234" || dc.URL != "https://example.com/device" {
		t.Fatalf("code: %+v", dc)
	}
	start := time.Now()
	token, err := dc.Wait(ctx)
	if err != nil || token != "gho_fromthedevice" {
		t.Fatalf("token %q, %v", token, err)
	}
	if polls != 3 || time.Since(start) < time.Second {
		t.Fatalf("asked %d times in %v: didn't slow down", polls, time.Since(start))
	}
	mu.Lock()
	deny = true
	mu.Unlock()
	dc, _ = StartDevice(ctx, d, "Iv1.test")
	if _, err := dc.Wait(ctx); !errors.Is(err, ErrDeviceDenied) {
		t.Fatalf("declined: %v", err)
	}
}
