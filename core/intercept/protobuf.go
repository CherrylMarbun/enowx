package intercept

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// DecodeProtobuf shows a protobuf message without its schema: each field
// by its number, with its wire type's value; length-delimited fields as
// text when they read as text, as a nested message when they parse as
// one, else as bytes. ok is false when data isn't protobuf.
func DecodeProtobuf(data []byte) (string, bool) {
	var b strings.Builder
	if !decodeMessage(&b, data, 0) {
		return "", false
	}
	return b.String(), true
}

const maxProtoDepth = 16

func decodeMessage(b *strings.Builder, data []byte, depth int) bool {
	if depth > maxProtoDepth {
		return false
	}
	indent := strings.Repeat("  ", depth)
	start := b.Len()
	for len(data) > 0 {
		key, n := binary.Uvarint(data)
		if n <= 0 || key>>3 == 0 {
			truncateBuilder(b, start)
			return false
		}
		data = data[n:]
		field, wire := key>>3, key&7
		switch wire {
		case 0: // varint
			v, n := binary.Uvarint(data)
			if n <= 0 {
				truncateBuilder(b, start)
				return false
			}
			data = data[n:]
			fmt.Fprintf(b, "%s%d: %d\n", indent, field, v)
		case 1: // 64-bit
			if len(data) < 8 {
				truncateBuilder(b, start)
				return false
			}
			v := binary.LittleEndian.Uint64(data)
			data = data[8:]
			fmt.Fprintf(b, "%s%d: %d (double %g)\n", indent, field, v, math.Float64frombits(v))
		case 5: // 32-bit
			if len(data) < 4 {
				truncateBuilder(b, start)
				return false
			}
			v := binary.LittleEndian.Uint32(data)
			data = data[4:]
			fmt.Fprintf(b, "%s%d: %d (float %g)\n", indent, field, v, math.Float32frombits(v))
		case 2: // length-delimited
			l, n := binary.Uvarint(data)
			if n <= 0 || uint64(len(data)-n) < l {
				truncateBuilder(b, start)
				return false
			}
			v := data[n : n+int(l)]
			data = data[n+int(l):]
			switch {
			case len(v) > 0 && utf8.Valid(v) && printable(v):
				fmt.Fprintf(b, "%s%d: %q\n", indent, field, v)
			default:
				mark := b.Len()
				fmt.Fprintf(b, "%s%d {\n", indent, field)
				if len(v) > 0 && decodeMessage(b, v, depth+1) {
					fmt.Fprintf(b, "%s}\n", indent)
				} else {
					truncateBuilder(b, mark)
					fmt.Fprintf(b, "%s%d: bytes %x\n", indent, field, v)
				}
			}
		default:
			truncateBuilder(b, start)
			return false
		}
	}
	return true
}

func printable(v []byte) bool {
	for _, r := range string(v) {
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			return false
		}
	}
	return true
}

// truncateBuilder cuts b back to n bytes.
func truncateBuilder(b *strings.Builder, n int) {
	s := b.String()[:n]
	b.Reset()
	b.WriteString(s)
}
