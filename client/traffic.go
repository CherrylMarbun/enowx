package client

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"

	"github.com/enowx/enowx/proto"
)

// The host's HTTP interceptor (docs/traffic.md).

func (c *Client) TrafficStatus() (proto.TrafficStatus, error) {
	var st proto.TrafficStatus
	err := c.Call(proto.MethodTrafficStatus, nil, &st)
	return st, err
}

func (c *Client) TrafficCA() (proto.TrafficCA, error) {
	var ca proto.TrafficCA
	err := c.Call(proto.MethodTrafficCA, nil, &ca)
	return ca, err
}

func (c *Client) TrafficAttach(p proto.TrafficAttachParams) (proto.TrafficTarget, error) {
	var t proto.TrafficTarget
	err := c.Call(proto.MethodTrafficAttach, p, &t)
	return t, err
}

func (c *Client) TrafficDetach(source string) error {
	return c.Call(proto.MethodTrafficDetach, proto.TrafficTargetRef{Source: source}, nil)
}

func (c *Client) TrafficAttached() ([]proto.TrafficTarget, error) {
	var ts []proto.TrafficTarget
	err := c.Call(proto.MethodTrafficAttached, nil, &ts)
	return ts, err
}

func (c *Client) TrafficEnv(p proto.TrafficEnvParams) (proto.TrafficEnv, error) {
	var e proto.TrafficEnv
	err := c.Call(proto.MethodTrafficEnv, p, &e)
	return e, err
}

func (c *Client) TrafficList(p proto.TrafficListParams) (proto.TrafficList, error) {
	var l proto.TrafficList
	err := c.Call(proto.MethodTrafficList, p, &l)
	return l, err
}

func (c *Client) TrafficGet(id int64, reveal bool) (proto.TrafficExchange, error) {
	var ex proto.TrafficExchange
	err := c.Call(proto.MethodTrafficGet, proto.TrafficRef{ID: id, Reveal: reveal}, &ex)
	return ex, err
}

func (c *Client) TrafficBody(p proto.TrafficBodyParams) (proto.TrafficBody, error) {
	var b proto.TrafficBody
	err := c.Call(proto.MethodTrafficBody, p, &b)
	return b, err
}

func (c *Client) TrafficMessages(p proto.TrafficMessagesParams) ([]proto.TrafficMessage, error) {
	var ms []proto.TrafficMessage
	err := c.Call(proto.MethodTrafficMessages, p, &ms)
	return ms, err
}

func (c *Client) TrafficPin(ids []int64, pinned bool) error {
	return c.Call(proto.MethodTrafficPin, proto.TrafficPinParams{IDs: ids, Pinned: pinned}, nil)
}

func (c *Client) TrafficClear(p proto.TrafficClearParams) error {
	return c.Call(proto.MethodTrafficClear, p, nil)
}

func (c *Client) TrafficRules(project string) (proto.TrafficRules, error) {
	var r proto.TrafficRules
	err := c.Call(proto.MethodTrafficRules, proto.TrafficRulesParams{Project: project}, &r)
	return r, err
}

func (c *Client) SetTrafficRules(r proto.TrafficRules) (proto.TrafficRules, error) {
	var out proto.TrafficRules
	err := c.Call(proto.MethodTrafficRulesSet, r, &out)
	return out, err
}

func (c *Client) TrafficPaused() ([]proto.TrafficPaused, error) {
	var ps []proto.TrafficPaused
	err := c.Call(proto.MethodTrafficPaused, nil, &ps)
	return ps, err
}

func (c *Client) TrafficResume(p proto.TrafficResumeParams) error {
	return c.Call(proto.MethodTrafficResume, p, nil)
}

func (c *Client) TrafficSend(p proto.TrafficSendParams) (proto.TrafficSummary, error) {
	var s proto.TrafficSummary
	err := c.Call(proto.MethodTrafficSend, p, &s)
	return s, err
}

// TrafficExport exports exchanges as HAR and reads it here.
func (c *Client) TrafficExport(p proto.TrafficExportParams) ([]byte, error) {
	var info proto.TransferInfo
	if err := c.Call(proto.MethodTrafficExport, p, &info); err != nil {
		return nil, err
	}
	defer c.DiscardTransfer(info.ID)
	var out []byte
	for int64(len(out)) < info.Size {
		part, err := c.ReadTransfer(info.ID, int64(len(out)))
		if err != nil {
			return nil, err
		}
		if len(part) == 0 {
			break
		}
		out = append(out, part...)
	}
	return out, nil
}

// TrafficImport sends a HAR to the host and imports it.
func (c *Client) TrafficImport(har []byte) (proto.TrafficList, error) {
	id := newImportID()
	for off := 0; off < len(har) || off == 0; {
		end := min(len(har), off+proto.TransferPartSize)
		if err := c.WriteTransfer(id, int64(off), har[off:end]); err != nil {
			return proto.TrafficList{}, err
		}
		off = end
		if off == len(har) {
			break
		}
	}
	var l proto.TrafficList
	err := c.Call(proto.MethodTrafficImport, proto.TrafficImportParams{ID: id}, &l)
	return l, err
}

func newImportID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (c *Client) TrafficSettings() (proto.TrafficSettings, error) {
	var st proto.TrafficSettings
	err := c.Call(proto.MethodTrafficSettings, nil, &st)
	return st, err
}

func (c *Client) SetTrafficSettings(st proto.TrafficSettings) (proto.TrafficSettings, error) {
	var out proto.TrafficSettings
	err := c.Call(proto.MethodTrafficSetSettings, st, &out)
	return out, err
}

// TrafficReverse puts a reverse proxy in front of a session's port, its
// traffic recorded as the session's Browser tool's.
func (c *Client) TrafficReverse(session string, port int) (proto.TrafficReverse, error) {
	var r proto.TrafficReverse
	err := c.Call(proto.MethodTrafficReverse, proto.TrafficReverseParams{Session: session, Port: port}, &r)
	return r, err
}

// OpenTrafficTunnel tunnels to a source's own port, or to one of its
// reverse proxies (port).
func (c *Client) OpenTrafficTunnel(source string, port int) (*Tunnel, error) {
	return c.openTunnel(proto.MethodTrafficTunnel, proto.TrafficTargetRef{Source: source, Port: port})
}

// AndroidProxy points a device of this host at a port of it, with the CA.
func (c *Client) AndroidProxy(p proto.AndroidProxyParams) (proto.AndroidProxyResult, error) {
	var r proto.AndroidProxyResult
	err := c.Call(proto.MethodAndroidProxy, p, &r)
	return r, err
}

// ForwardTrafficSource listens on this machine's loopback and carries each
// connection to source's own port on the daemon's host: for a device here
// to send through the interceptor there.
func (c *Client) ForwardTrafficSource(source string) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				t, err := c.OpenTrafficTunnel(source, 0)
				if err != nil {
					return
				}
				defer t.Close()
				done := make(chan struct{})
				go func() { io.Copy(t, conn); t.Close(); close(done) }()
				io.Copy(conn, t)
				conn.Close()
				<-done
			}()
		}
	}()
	return ln, nil
}
