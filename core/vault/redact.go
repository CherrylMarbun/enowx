package vault

import (
	"bytes"
	"encoding/base64"
	"io"
	"slices"
	"strings"
	"sync"
)

// Mask hides a value but its last four characters, "•••last4"; short
// values hide whole.
func Mask(v string) string {
	if len(v) < 12 {
		return "•••"
	}
	return "•••" + v[len(v)-4:]
}

// Redactor writes what goes through it with the secrets masked, as a
// command run with credentials prints to an agent: a secret split between
// two writes is masked too, the end held back until Close.
type Redactor struct {
	mu      sync.Mutex
	w       io.Writer
	secrets [][]byte
	masks   [][]byte
	pending []byte
	longest int
}

// NewRedactor masks secrets (and their base64, as in basic auth) in what
// it writes to w. Values under 6 bytes are left: masking them would mask
// ordinary words.
func NewRedactor(w io.Writer, secrets []string) *Redactor {
	r := &Redactor{w: w}
	add := func(s, mask string) {
		if len(s) < 6 || slices.ContainsFunc(r.secrets, func(o []byte) bool { return string(o) == s }) {
			return
		}
		r.secrets, r.masks = append(r.secrets, []byte(s)), append(r.masks, []byte(mask))
		r.longest = max(r.longest, len(s))
	}
	for _, s := range secrets {
		s = strings.TrimSpace(s)
		add(s, Mask(s))
		add(base64.StdEncoding.EncodeToString([]byte(s)), Mask(s))
	}
	// The longest first, so a secret holding another masks whole.
	idx := make([]int, len(r.secrets))
	for i := range idx {
		idx[i] = i
	}
	slices.SortFunc(idx, func(a, b int) int { return len(r.secrets[b]) - len(r.secrets[a]) })
	s, m := make([][]byte, len(idx)), make([][]byte, len(idx))
	for i, j := range idx {
		s[i], m[i] = r.secrets[j], r.masks[j]
	}
	r.secrets, r.masks = s, m
	return r
}

// Write masks and writes p, holding back what could start a secret.
func (r *Redactor) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.secrets) == 0 {
		return r.w.Write(p)
	}
	r.pending = r.mask(append(r.pending, p...))
	keep := min(len(r.pending), r.longest-1)
	out := r.pending[:len(r.pending)-keep]
	if len(out) > 0 {
		if _, err := r.w.Write(out); err != nil {
			return 0, err
		}
	}
	r.pending = append([]byte(nil), r.pending[len(r.pending)-keep:]...)
	return len(p), nil
}

// Close writes what was held back.
func (r *Redactor) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) == 0 {
		return nil
	}
	_, err := r.w.Write(r.mask(r.pending))
	r.pending = nil
	return err
}

func (r *Redactor) mask(b []byte) []byte {
	for i, s := range r.secrets {
		if bytes.Contains(b, s) {
			b = bytes.ReplaceAll(b, s, r.masks[i])
		}
	}
	return b
}

// Redact masks secrets in s.
func Redact(s string, secrets []string) string {
	var b bytes.Buffer
	r := NewRedactor(&b, secrets)
	_, _ = r.Write([]byte(s))
	_ = r.Close()
	return b.String()
}
