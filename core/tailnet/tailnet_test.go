package tailnet

import (
	"strings"
	"testing"
)

func TestParseStatus(t *testing.T) {
	data := []byte(`{
		"BackendState": "Running",
		"Self": {"HostName": "my-mac"},
		"Peer": {
			"nodekey:a": {"HostName": "vps-sg", "DNSName": "vps-sg.tail1234.ts.net.", "OS": "linux", "TailscaleIPs": ["100.64.0.2", "fd7a::2"], "Online": false},
			"nodekey:b": {"HostName": "build-box", "DNSName": "build-box.tail1234.ts.net.", "OS": "linux", "TailscaleIPs": ["100.64.0.3"], "Online": true},
			"nodekey:c": {"HostName": "", "DNSName": "phone.tail1234.ts.net.", "OS": "iOS", "TailscaleIPs": ["100.64.0.4"], "Online": true}
		}
	}`)
	peers, err := parseStatus(data)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range peers {
		names = append(names, p.HostName)
	}
	// Online first, then by name; a missing HostName comes from DNSName.
	want := []string{"build-box", "phone", "vps-sg"}
	if len(names) != len(want) {
		t.Fatalf("peers = %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("peers = %v, want %v", names, want)
		}
	}
	if got := peers[2].Address(); got != "vps-sg.tail1234.ts.net" {
		t.Fatalf("address = %q", got)
	}
	if peers[2].IP != "100.64.0.2" {
		t.Fatalf("ip = %q", peers[2].IP)
	}
}

func TestParseStatusStopped(t *testing.T) {
	if _, err := parseStatus([]byte(`{"BackendState": "Stopped"}`)); err == nil {
		t.Fatal("want an error when Tailscale is stopped")
	}
}

func TestParseStatusSkipsALeadingNotice(t *testing.T) {
	data := []byte("Warning: client version \"1.80\" != tailscaled server version \"1.82\"\n" +
		`{"BackendState":"Running","Peer":{"k":{"HostName":"box","DNSName":"box.ts.net.","Online":true}}}`)
	peers, err := parseStatus(data)
	if err != nil || len(peers) != 1 || peers[0].HostName != "box" {
		t.Fatalf("peers = %+v, err = %v", peers, err)
	}
	_, err = parseStatus([]byte("The Tailscale CLI failed to start"))
	if err == nil || !strings.Contains(err.Error(), "failed to start") {
		t.Fatalf("err = %v, want the CLI's own words", err)
	}
}
