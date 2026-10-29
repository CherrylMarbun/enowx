// Package discordrpc shows what the user does in Discord, as Rich
// Presence: it talks to the Discord app running on the same device over
// its local IPC socket (a Unix socket, or a named pipe on Windows). Nothing
// leaves the device but through Discord, and only while it runs.
package discordrpc

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// Activity is what Discord shows under the user's name.
type Activity struct {
	Details    string   `json:"details,omitempty"` // the first line
	State      string   `json:"state,omitempty"`   // the second
	Timestamps *Times   `json:"timestamps,omitempty"`
	Assets     *Assets  `json:"assets,omitempty"`
	Buttons    []Button `json:"buttons,omitempty"` // two at most
	Type       int      `json:"type,omitempty"`    // 0: Playing
}

// Times shows how long the activity has gone on, from Start (Unix
// seconds).
type Times struct {
	Start int64 `json:"start,omitempty"`
}

// Assets are the images, by the names they were uploaded with to the
// Discord application, and their tooltips.
type Assets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

// Button is a link under the activity.
type Button struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Equal says a and b show the same.
func Equal(a, b *Activity) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

const (
	opHandshake = 0
	opFrame     = 1
	opClose     = 2
)

// ErrNotRunning: no Discord app is running on this device.
var ErrNotRunning = errors.New("discordrpc: Discord isn't running")

// Client is a connection to the Discord app, for one application. Its
// methods may be called from any goroutine.
type Client struct {
	mu   sync.Mutex
	conn io.ReadWriteCloser
}

// Dial connects to the Discord app as application appID.
func Dial(appID string) (*Client, error) {
	conn, err := dial()
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn}
	if err := c.send(opHandshake, map[string]any{"v": 1, "client_id": appID}); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := c.read(); err != nil { // READY, or an error for a wrong ID
		conn.Close()
		return nil, err
	}
	return c, nil
}

// SetActivity shows act; nil clears it.
func (c *Client) SetActivity(act *Activity) error {
	var b [8]byte
	rand.Read(b[:])
	args := map[string]any{"pid": os.Getpid(), "activity": act}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendLocked(opFrame, map[string]any{"cmd": "SET_ACTIVITY", "args": args, "nonce": hex.EncodeToString(b[:])}); err != nil {
		return err
	}
	reply, err := c.readLocked()
	if err != nil {
		return err
	}
	var r struct {
		Evt  string `json:"evt"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if json.Unmarshal(reply, &r) == nil && r.Evt == "ERROR" {
		return fmt.Errorf("discordrpc: %s", r.Data.Message)
	}
	return nil
}

// Close clears the activity and disconnects.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.sendLocked(opClose, map[string]any{})
	return c.conn.Close()
}

func (c *Client) send(op uint32, v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendLocked(op, v)
}

func (c *Client) read() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readLocked()
}

// A frame is its opcode and length, little-endian, then its JSON.
func (c *Client) sendLocked(op uint32, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	frame := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(frame[0:], op)
	binary.LittleEndian.PutUint32(frame[4:], uint32(len(data)))
	copy(frame[8:], data)
	_, err = c.conn.Write(frame)
	return err
}

func (c *Client) readLocked() ([]byte, error) {
	var head [8]byte
	if _, err := io.ReadFull(c.conn, head[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(head[4:])
	if n > 1<<20 {
		return nil, errors.New("discordrpc: a frame too large")
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint32(head[0:]) == opClose {
		return nil, fmt.Errorf("discordrpc: Discord closed: %s", data)
	}
	return data, nil
}
