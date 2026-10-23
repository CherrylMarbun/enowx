// Package transcript reads the conversations agents record on disk, so the
// app can show a session as a chat next to its terminal. A Reader follows
// one file as the agent appends to it.
package transcript

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/enowx/enowx/proto"
)

// parser turns one recorded line into changes to the entries.
type parser interface {
	parse(line []byte, r *Reader)
}

// Reader follows a transcript file. It isn't safe for concurrent use.
type Reader struct {
	path   string
	parser parser
	offset int64
	rest   []byte // a line the agent hasn't finished writing

	entries []proto.TranscriptEntry
	rev     int64
	tools   map[string]int // tool call ID → entry index

	// The agent's settings, as the transcript last said them.
	Model, Mode, Effort string
	// Context is the tokens the conversation filled at the agent's last
	// answer; Window how much the model holds, when the agent says.
	Context, Window int64
}

func newReader(path string, p parser) *Reader {
	return &Reader{path: path, parser: p, tools: map[string]int{}}
}

// Path is the file the reader follows.
func (r *Reader) Path() string { return r.path }

// Update reads what the agent appended since the last call.
func (r *Reader) Update() error {
	f, err := os.Open(r.path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() < r.offset {
		// Rewritten from scratch: read it again.
		*r = *newReader(r.path, r.parser)
		r.rev = 1
	}
	if _, err := f.Seek(r.offset, io.SeekStart); err != nil {
		return err
	}
	br := bufio.NewReaderSize(f, 64<<10)
	for {
		line, err := br.ReadBytes('\n')
		r.offset += int64(len(line))
		if errors.Is(err, io.EOF) {
			r.rest = append(r.rest, line...)
			return nil
		}
		if err != nil {
			return err
		}
		if len(r.rest) > 0 {
			line = append(r.rest, line...)
			r.rest = nil
		}
		if line = bytes.TrimSpace(line); len(line) > 0 {
			r.parser.parse(line, r)
		}
	}
}

// Since returns the entries added or changed after revision since, and
// the latest revision.
func (r *Reader) Since(since int64) ([]proto.TranscriptEntry, int64) {
	var out []proto.TranscriptEntry
	for _, e := range r.entries {
		if e.Rev > since {
			out = append(out, e)
		}
	}
	return out, r.rev
}

// Before returns about n entries ending before index end (all the rest when
// end is past the last), starting at a message of the user's so that the
// first turn is whole, and the latest revision.
func (r *Reader) Before(end, n int) ([]proto.TranscriptEntry, int64) {
	end = min(max(end, 0), len(r.entries))
	start := max(end-n, 0)
	for start > 0 && r.entries[start].Kind != proto.TranscriptUser {
		start--
	}
	return append([]proto.TranscriptEntry(nil), r.entries[start:end]...), r.rev
}

// Len is how many entries the transcript has.
func (r *Reader) Len() int { return len(r.entries) }

func (r *Reader) add(e proto.TranscriptEntry) int {
	r.rev++
	e.Index, e.Rev = len(r.entries), r.rev
	r.entries = append(r.entries, e)
	return e.Index
}

func (r *Reader) change(i int, fn func(*proto.TranscriptEntry)) {
	r.rev++
	fn(&r.entries[i])
	r.entries[i].Rev = r.rev
}

// clip shortens long tool output; the terminal has all of it.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && s[cut]&0xC0 == 0x80 { // stay on a character boundary
		cut--
	}
	return s[:cut] + "\n…"
}
