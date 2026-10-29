//go:build !windows

package discordrpc

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// The client finds Discord's socket, says which application it is, and
// sets the activity Discord shows; with no Discord, it says so.
func TestSetsTheActivity(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "dipc") // short: a socket's path has a limit
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	for _, env := range []string{"XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		t.Setenv(env, dir)
	}
	if _, err := Dial("123"); err != ErrNotRunning {
		t.Fatalf("no Discord: %v", err)
	}
	l, err := net.Listen("unix", filepath.Join(dir, "discord-ipc-0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	got := make(chan map[string]any, 4)
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		for {
			var head [8]byte
			if _, err := io.ReadFull(c, head[:]); err != nil {
				return
			}
			data := make([]byte, binary.LittleEndian.Uint32(head[4:]))
			io.ReadFull(c, data)
			var m map[string]any
			json.Unmarshal(data, &m)
			got <- m
			reply, _ := json.Marshal(map[string]any{"cmd": m["cmd"], "evt": "READY"})
			binary.LittleEndian.PutUint32(head[0:], opFrame)
			binary.LittleEndian.PutUint32(head[4:], uint32(len(reply)))
			c.Write(append(head[:], reply...))
		}
	}()

	c, err := Dial("123")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if hello := <-got; hello["client_id"] != "123" {
		t.Fatalf("handshake: %v", hello)
	}
	act := &Activity{Details: "Working on enowx · main", State: "Claude Code · 2 sessions", Buttons: []Button{{Label: "Get enowX", URL: "https://enowx.ai"}}}
	if err := c.SetActivity(act); err != nil {
		t.Fatal(err)
	}
	set := <-got
	args, _ := set["args"].(map[string]any)
	sent, _ := args["activity"].(map[string]any)
	buttons, _ := sent["buttons"].([]any)
	if set["cmd"] != "SET_ACTIVITY" || sent["details"] != act.Details || len(buttons) != 1 {
		t.Fatalf("what Discord got: %v", set)
	}
}
