package vps

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// InstaBox, against a fake of its API answering as its documentation
// shows: the key checked, the sizes the plan allows, a machine of the size
// picked made and waited for until it runs, its gateway's address, a
// certificate for the user's key, the machines the user has, one paused by
// hand resumed before its certificate, and deleting it, every POST and
// DELETE with an Idempotency-Key.
func TestInstaBoxMakesAMachineReachedByCertificate(t *testing.T) {
	var created map[string]any
	polls := 0
	oldState := "suspended"
	wakePoll = time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ibk_test" {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"error":{"code":"invalid_key","message":"invalid API key"}}`)
			return
		}
		if (r.Method == "POST" || r.Method == "DELETE") && r.Header.Get("Idempotency-Key") == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":{"code":"idempotency_key_required","message":"Idempotency-Key is required"}}`)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /me":
			io.WriteString(w, `{"login":"ada@example.com"}`)
		case "GET /orgs":
			io.WriteString(w, `{"orgs":[{"id":"org_1","tier":"free","allowedSpecs":[{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20}]}]}`)
		case "GET /machines":
			io.WriteString(w, `{"machines":[{"id":"m_old","name":"enowx","state":"`+oldState+`","runtime":{"ready":false},"spec":{"vcpus":4,"memMiB":8192}}],"nextCursor":null}`)
		case "GET /machines/m_old":
			ready := oldState == "running"
			io.WriteString(w, `{"id":"m_old","name":"enowx","state":"`+oldState+`","runtime":{"ready":`+map[bool]string{true: "true", false: "false"}[ready]+`}}`)
		case "POST /machines/m_old/resume":
			oldState = "running"
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"op_3","machineId":"m_old","type":"resume","state":"queued"}`)
		case "POST /machines/m_old/connections":
			if oldState != "running" {
				t.Error("a certificate for a machine still paused")
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"certificate":"ssh-ed25519-cert-v01@openssh.com AAAAold","hostTrust":{"publicKey":"ssh-ed25519 AAAAhost"}}`)
		case "POST /machines":
			json.NewDecoder(r.Body).Decode(&created)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"op_1","machineId":"m_box1","type":"create","state":"queued"}`)
		case "GET /machines/m_box1":
			polls++
			state := `"starting","runtime":{"ready":false}`
			if polls > 1 {
				state = `"running","runtime":{"ready":true}`
			}
			io.WriteString(w, `{"id":"m_box1","name":"enowx-1","state":`+state+`}`)
		case "GET /machines/m_box1/connection-info":
			io.WriteString(w, `{"machineId":"m_box1","host":"ssh.instabox.sh","port":2222,"username":"m_box1","hostTrust":{"type":"ca","publicKey":"ssh-ed25519 AAAAhost"}}`)
		case "POST /machines/m_box1/connections":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["publicKey"] != userKey {
				t.Errorf("certificate for %v", body["publicKey"])
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"id":"cn_1","host":"ssh.instabox.sh","port":2222,"username":"m_box1","certificate":"ssh-ed25519-cert-v01@openssh.com AAAAcert","hostTrust":{"type":"ca","publicKey":"ssh-ed25519 AAAAhost"}}`)
		case "DELETE /machines/m_box1":
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"op_2","machineId":"m_box1","type":"delete","state":"queued"}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	ctx := context.Background()

	if bad, _ := New("instabox", "ibk_wrong", srv.URL); bad != nil {
		if _, err := bad.Regions(ctx); err == nil || !strings.Contains(err.Error(), "refused") {
			t.Fatalf("a wrong key: %v", err)
		}
	}
	c, err := New("instabox", "ibk_test", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	regions, err := c.Regions(ctx)
	if err != nil || len(regions) != 1 {
		t.Fatalf("regions: %v %v", regions, err)
	}
	sizes, _ := c.Sizes(ctx, regions[0].ID)
	if len(sizes) != 1 || sizes[0].ID != "4/8192" {
		t.Fatalf("the Free plan's sizes: %+v", sizes)
	}
	s, err := c.Create(ctx, CreateParams{Name: "enowX 1", Region: regions[0].ID, Size: sizes[0].ID, PublicKey: userKey})
	if err != nil || s.ID != "m_box1" {
		t.Fatalf("create: %+v %v", s, err)
	}
	if created["name"] != "enowx-1" || created["vcpus"] != float64(4) || created["memMiB"] != float64(8192) {
		t.Fatalf("what was asked for: %v", created)
	}
	s, err = WaitReady(ctx, c, s.ID, time.Millisecond, nil)
	if err != nil || s.SSH != "m_box1@ssh.instabox.sh:2222" || s.IPv4 != "" {
		t.Fatalf("ready: %+v %v", s, err)
	}
	if !NeedsCertificate("instabox") || NeedsCertificate("hetzner") {
		t.Fatal("which providers need a certificate")
	}
	cert, err := Certificate(ctx, "instabox", "ibk_test", srv.URL, "m_box1", userKey+"\n")
	if err != nil || !strings.HasPrefix(cert.Certificate, "ssh-ed25519-cert-v01") || cert.HostAuthority != "ssh-ed25519 AAAAhost" {
		t.Fatalf("certificate: %+v %v", cert, err)
	}
	mine, err := c.(Lister).Servers(ctx)
	if err != nil || len(mine) != 1 || mine[0].ID != "m_old" || mine[0].Ready || mine[0].Size != "4 vCPUs, 8 GB" {
		t.Fatalf("the user's machines: %+v %v", mine, err)
	}
	if cert, err := Certificate(ctx, "instabox", "ibk_test", srv.URL, "m_old", userKey); err != nil || cert.Certificate == "" || oldState != "running" {
		t.Fatalf("a paused machine's certificate: %+v %v (%s)", cert, err, oldState)
	}
	if err := c.Delete(ctx, "m_box1"); err != nil {
		t.Fatal(err)
	}
}
