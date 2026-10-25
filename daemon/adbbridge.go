package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/enowx/enowx/core/android"
	"github.com/enowx/enowx/proto"
)

// The ADB bridge (proto/android.go): the device's host opens tunnels to
// its ADB server; the session's host listens on loopback for its tools and
// hands each connection to the client as a tunnel. Listeners and the
// connections waiting to be taken belong to their client connection, and
// close with it.

// adbPort is where adb's server listens (a test points it elsewhere).
var adbPort = "127.0.0.1:5037"

// adbAcceptWait is how long an accepted connection waits for the client
// to take it.
const adbAcceptWait = 30 * time.Second

type adbBridge struct {
	listeners map[uint32]net.Listener
	pending   map[uint32]net.Conn
	next      uint32
}

func (c *conn) adbOpen() (proto.TunnelInfo, error) {
	sdk, ok := android.FindSDK()
	if !ok {
		return proto.TunnelInfo{}, errors.New("android: no Android SDK on this host")
	}
	sdk.StartServer()
	nc, err := net.Dial("tcp", adbPort)
	if err != nil {
		return proto.TunnelInfo{}, err
	}
	id, err := c.attachTunnel(nc)
	return proto.TunnelInfo{Stream: id}, err
}

// forwardOpen tunnels to a port adb forwards to a device here, and to no
// other port.
func (c *conn) forwardOpen(p proto.AndroidForwardParams) (proto.TunnelInfo, error) {
	sdk, ok := android.FindSDK()
	if !ok {
		return proto.TunnelInfo{}, errors.New("android: no Android SDK on this host")
	}
	list, err := sdk.ADBRun("", "forward", "--list")
	if err != nil {
		return proto.TunnelInfo{}, err
	}
	want := " tcp:" + strconv.Itoa(p.Port) + " "
	if !strings.Contains(list, want) {
		return proto.TunnelInfo{}, fmt.Errorf("android: adb doesn't forward port %d here", p.Port)
	}
	nc, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(p.Port))
	if err != nil {
		return proto.TunnelInfo{}, err
	}
	id, err := c.attachTunnel(nc)
	return proto.TunnelInfo{Stream: id}, err
}

func (c *conn) adbListen() (proto.AndroidADBListener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return proto.AndroidADBListener{}, err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		ln.Close()
		return proto.AndroidADBListener{}, errors.New("connection closed")
	}
	b := &c.adb
	if b.listeners == nil {
		b.listeners, b.pending = map[uint32]net.Listener{}, map[uint32]net.Conn{}
	}
	b.next++
	id := b.next
	b.listeners[id] = ln
	c.mu.Unlock()
	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			c.mu.Lock()
			b.next++
			cid := b.next
			b.pending[cid] = nc
			c.mu.Unlock()
			time.AfterFunc(adbAcceptWait, func() {
				c.mu.Lock()
				left, ok := b.pending[cid]
				delete(b.pending, cid)
				c.mu.Unlock()
				if ok {
					left.Close()
				}
			})
			c.event(proto.EventAndroidADBAccepted, proto.AndroidADBConn{Listener: id, ID: cid})
		}
	}()
	return proto.AndroidADBListener{ID: id, Port: ln.Addr().(*net.TCPAddr).Port}, nil
}

func (c *conn) adbAccept(p proto.AndroidADBConn) (proto.TunnelInfo, error) {
	c.mu.Lock()
	nc, ok := c.adb.pending[p.ID]
	delete(c.adb.pending, p.ID)
	c.mu.Unlock()
	if !ok {
		return proto.TunnelInfo{}, errors.New("android: that connection is gone")
	}
	id, err := c.attachTunnel(nc)
	return proto.TunnelInfo{Stream: id}, err
}

func (c *conn) adbClose(id uint32) {
	c.mu.Lock()
	ln := c.adb.listeners[id]
	delete(c.adb.listeners, id)
	c.mu.Unlock()
	if ln != nil {
		ln.Close()
	}
}

// closeADB closes the bridge's listeners and waiting connections.
func (c *conn) closeADB() {
	c.mu.Lock()
	b := c.adb
	c.adb = adbBridge{}
	c.mu.Unlock()
	for _, ln := range b.listeners {
		ln.Close()
	}
	for _, nc := range b.pending {
		nc.Close()
	}
}

// event sends an event to this connection only.
func (c *conn) event(name string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	ev, err := json.Marshal(proto.Event{Name: name, Data: raw})
	if err != nil {
		return
	}
	c.send(proto.Frame{Type: proto.TypeEvent, Payload: ev})
}
