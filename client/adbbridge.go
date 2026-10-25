package client

import (
	"encoding/json"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/enowx/enowx/proto"
)

// ADBBridge makes one host's ADB server reachable on another's loopback,
// for a session there to use a device here (proto/android.go): each
// connection its tools make is carried through this client, between the
// two daemons.
type ADBBridge struct {
	// Addr is the address on the session's host, for ADB_SERVER_SOCKET
	// (tcp:Addr).
	Addr     string
	session  *Client
	listener uint32
	once     sync.Once
}

// BridgeADB bridges device's ADB server to session's host. The same
// client may be both, when the device is on the session's host, but then
// no bridge is needed.
func BridgeADB(session, device *Client) (*ADBBridge, error) {
	var l proto.AndroidADBListener
	if err := session.Call(proto.MethodAndroidADBListen, nil, &l); err != nil {
		return nil, err
	}
	b := &ADBBridge{Addr: "127.0.0.1:" + strconv.Itoa(l.Port), session: session, listener: l.ID}
	session.mu.Lock()
	if session.adb == nil {
		session.adb = map[uint32]func(uint32){}
	}
	session.adb[l.ID] = func(id uint32) { go relayADB(session, device, id) }
	session.mu.Unlock()
	return b, nil
}

// Close stops the bridge: new connections are refused, those carried go
// on until they end.
func (b *ADBBridge) Close() {
	b.once.Do(func() {
		b.session.mu.Lock()
		delete(b.session.adb, b.listener)
		b.session.mu.Unlock()
		_ = b.session.Call(proto.MethodAndroidADBClose, proto.AndroidADBListener{ID: b.listener}, nil)
	})
}

// adbAccepted hands a connection accepted on a bridge to its relay.
func (c *Client) adbAccepted(ev proto.Event) {
	var a proto.AndroidADBConn
	if json.Unmarshal(ev.Data, &a) != nil {
		return
	}
	c.mu.Lock()
	fn := c.adb[a.Listener]
	c.mu.Unlock()
	if fn != nil {
		fn(a.ID)
	}
}

// relayADB carries connection id of the session's host to a new
// connection to the device's host's ADB server.
func relayADB(session, device *Client, id uint32) {
	from, err := session.openTunnel(proto.MethodAndroidADBAccept, proto.AndroidADBConn{ID: id})
	if err != nil {
		return
	}
	defer from.Close()
	to, err := device.openTunnel(proto.MethodAndroidADBOpen, nil)
	if err != nil {
		return
	}
	defer to.Close()
	done := make(chan struct{})
	go func() {
		io.Copy(to, from)
		to.Close()
		close(done)
	}()
	io.Copy(from, to)
	from.Close()
	<-done
}

// ForwardADBPort listens on this machine's loopback port and carries each
// connection to the same port on device's host, which adb forwards to a
// device there (scrcpy's video). Close the listener to stop.
func ForwardADBPort(device *Client, port int) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				t, err := device.openTunnel(proto.MethodAndroidForwardOpen, proto.AndroidForwardParams{Port: port})
				if err != nil {
					return
				}
				defer t.Close()
				done := make(chan struct{})
				go func() { io.Copy(t, c); t.Close(); close(done) }()
				io.Copy(c, t)
				c.Close()
				<-done
			}()
		}
	}()
	return ln, nil
}
