package proto

import "encoding/binary"

// MethodTunnelOpen connects to a TCP port a session's processes listen on,
// on the daemon's host: TunnelOpenParams → TunnelInfo. Its bytes go in
// TypeTunnelData frames on the tunnel's channel, both ways, each side
// sending at most TunnelWindow bytes the other hasn't acknowledged
// (TypeTunnelAck), so a large transfer can't hold up terminal frames.
//
// Clients send tunnel frames only on a channel tunnel.open gave them: a
// daemon without tunnels answers it as an unknown method, and never sees
// one.
const MethodTunnelOpen = "tunnel.open"

// Tunnel flow: a side sends at most TunnelWindow bytes not acknowledged,
// in parts of at most TunnelChunk.
const (
	TunnelWindow = 256 << 10
	TunnelChunk  = 32 << 10
)

// TunnelOpenParams names the port, of session Session.
type TunnelOpenParams struct {
	Session string `json:"session"`
	Port    int    `json:"port"`
}

// TunnelInfo names the tunnel's channel.
type TunnelInfo struct {
	Stream uint32 `json:"stream"`
}

// EncodeAck is a TypeTunnelAck payload for n bytes.
func EncodeAck(n int) []byte { return binary.BigEndian.AppendUint32(nil, uint32(n)) }

// DecodeAck reads a TypeTunnelAck payload.
func DecodeAck(p []byte) int {
	if len(p) < 4 {
		return 0
	}
	return int(binary.BigEndian.Uint32(p))
}
