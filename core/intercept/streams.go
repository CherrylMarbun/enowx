package intercept

import (
	"bytes"
	"encoding/binary"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// streamParser returns what splits a body of contentType into messages
// as it streams (SSE events, gRPC messages), nil for other bodies.
func streamParser(ex *Exchange, sent bool, contentType string) func([]byte) {
	switch kindOf(contentType) {
	case "sse":
		if sent {
			return nil
		}
		return sseParser(ex)
	case "grpc":
		return grpcParser(ex, sent)
	}
	return nil
}

// sseParser splits server-sent events: fields until a blank line.
func sseParser(ex *Exchange) func([]byte) {
	var buf bytes.Buffer
	return func(p []byte) {
		buf.Write(p)
		for {
			data := buf.Bytes()
			i, n := eventEnd(data)
			if i < 0 {
				if buf.Len() > 1<<20 { // a runaway event: keep what is there
					i, n = buf.Len(), 0
				} else {
					return
				}
			}
			event := string(data[:i])
			buf.Next(i + n)
			var name string
			var lines []string
			for _, line := range strings.Split(strings.ReplaceAll(event, "\r\n", "\n"), "\n") {
				field, value, _ := strings.Cut(line, ":")
				value = strings.TrimPrefix(value, " ")
				switch field {
				case "event":
					name = value
				case "data":
					lines = append(lines, value)
				}
			}
			if len(lines) == 0 && name == "" {
				continue
			}
			text := strings.Join(lines, "\n")
			ex.addMessage(proto.TrafficMessage{Time: time.Now(), Kind: "event", Event: name, Data: []byte(text), Size: int64(len(text))})
		}
	}
}

func eventEnd(b []byte) (int, int) {
	for _, sep := range []string{"\r\n\r\n", "\n\n"} {
		if i := bytes.Index(b, []byte(sep)); i >= 0 {
			return i, len(sep)
		}
	}
	return -1, 0
}

// grpcParser splits gRPC's length-prefixed messages: a compressed flag,
// a 4-byte length, the message.
func grpcParser(ex *Exchange, sent bool) func([]byte) {
	var buf bytes.Buffer
	return func(p []byte) {
		buf.Write(p)
		for buf.Len() >= 5 {
			data := buf.Bytes()
			n := int(binary.BigEndian.Uint32(data[1:5]))
			if buf.Len() < 5+n {
				if n > 64<<20 { // not gRPC after all
					buf.Reset()
				}
				return
			}
			msg := append([]byte(nil), data[5:5+n]...)
			kind := "message"
			if data[0]&1 == 1 {
				kind = "compressed"
			}
			buf.Next(5 + n)
			ex.addMessage(proto.TrafficMessage{Sent: sent, Time: time.Now(), Kind: kind, Data: msg, Size: int64(n)})
		}
	}
}
