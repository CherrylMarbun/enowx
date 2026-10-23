package chatview

import (
	"testing"

	"github.com/enowx/enowx/proto"
)

func entries(from, to int) []proto.TranscriptEntry {
	var out []proto.TranscriptEntry
	for i := from; i < to; i++ {
		kind := proto.TranscriptAssistant
		if i%3 == 0 {
			kind = proto.TranscriptUser
		}
		out = append(out, proto.TranscriptEntry{Index: i, Kind: kind, Added: 1})
	}
	return out
}

// The chat opens on the last entries, takes changes to them, and puts
// older ones in front as they load.
func TestChatPagesInOlderEntries(t *testing.T) {
	ch := New()
	ch.apply(proto.Transcript{Supported: true, Entries: entries(6, 9), Rev: 9})
	if ch.base != 6 || len(ch.entries) != 3 || ch.animateFrom != 9 || len(ch.turns) != 1 {
		t.Fatalf("opened on base %d, %d entries, animate from %d, %d turns", ch.base, len(ch.entries), ch.animateFrom, len(ch.turns))
	}
	// A change to a shown entry, one to an entry not loaded, and a new one.
	ch.apply(proto.Transcript{Supported: true, Rev: 11, Entries: []proto.TranscriptEntry{
		{Index: 7, Kind: proto.TranscriptAssistant, Text: "changed"}, {Index: 2, Text: "old"}, {Index: 9, Kind: proto.TranscriptUser},
	}})
	if len(ch.entries) != 4 || ch.entries[1].Text != "changed" || len(ch.turns) != 2 {
		t.Fatalf("entries = %+v", ch.entries)
	}
	ch.prepend(entries(3, 6))
	if ch.base != 3 || len(ch.entries) != 7 || ch.entries[0].Index != 3 || len(ch.turns) != 3 {
		t.Fatalf("after older: base %d, %+v", ch.base, ch.entries)
	}
	// An answer that isn't what comes right before is left out.
	ch.prepend(entries(0, 2))
	if ch.base != 3 {
		t.Fatalf("took a gap: base %d", ch.base)
	}
	// An older daemon answers with everything: the part before is taken.
	ch.prepend(entries(0, 9))
	if ch.base != 0 || len(ch.entries) != 10 {
		t.Fatalf("from an older daemon: base %d, %d entries", ch.base, len(ch.entries))
	}
}

func TestNewsSkipsQuietPolls(t *testing.T) {
	last := proto.Transcript{Supported: true, Rev: 5, Model: "m"}
	if news(last, proto.Transcript{Supported: true, Rev: 5, Model: "m"}) {
		t.Fatal("a quiet poll is news")
	}
	if !news(last, proto.Transcript{Supported: true, Rev: 5, Model: "m", ContextTokens: 10}) {
		t.Fatal("a context change isn't news")
	}
}
