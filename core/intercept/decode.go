package intercept

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// decodeLimit bounds a decoded body, so a small bomb can't fill memory.
const decodeLimit = 64 << 20

// decode undoes a body's content encodings (gzip, deflate, br, zstd), the
// last applied first; a body it can't decode comes back as it was, with
// ok false.
func decode(data []byte, encoding string) (out []byte, ok bool) {
	encs := strings.Split(encoding, ",")
	out = data
	for i := len(encs) - 1; i >= 0; i-- {
		var r io.Reader
		var err error
		switch strings.ToLower(strings.TrimSpace(encs[i])) {
		case "", "identity":
			continue
		case "gzip", "x-gzip":
			r, err = gzip.NewReader(bytes.NewReader(out))
		case "deflate":
			// Servers send zlib or raw deflate under this name.
			if r, err = zlib.NewReader(bytes.NewReader(out)); err != nil {
				r, err = flate.NewReader(bytes.NewReader(out)), nil
			}
		case "br":
			r = brotli.NewReader(bytes.NewReader(out))
		case "zstd":
			var d *zstd.Decoder
			if d, err = zstd.NewReader(bytes.NewReader(out)); err == nil {
				defer d.Close()
				r = d
			}
		default:
			return data, false
		}
		if err != nil {
			return data, false
		}
		dec, err := io.ReadAll(io.LimitReader(r, decodeLimit))
		if err != nil && len(dec) == 0 {
			return data, false
		}
		out = dec
	}
	return out, true
}
