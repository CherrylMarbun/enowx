package seal

import (
	"bytes"
	"errors"
	"testing"
)

// A team key sealed to a device opens there only; an item sealed with it
// opens only as the item and version it was sealed as.
func TestTeamKeyAndItems(t *testing.T) {
	alice, _ := NewDeviceKey()
	bob, _ := NewDeviceKey()
	key, err := NewTeamKey()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := SealKey(key, alice.Public())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, key) {
		t.Fatal("the team key shows in what the cloud keeps")
	}
	again, err := LoadDeviceKey(alice.Private())
	if err != nil {
		t.Fatal(err)
	}
	got, err := again.OpenKey(sealed)
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("alice's device: %v", err)
	}
	if _, err := bob.OpenKey(sealed); !errors.Is(err, ErrOpen) {
		t.Fatalf("bob opened alice's key: %v", err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := alice.OpenKey(sealed); !errors.Is(err, ErrOpen) {
		t.Fatalf("a changed key opened: %v", err)
	}

	note := []byte("# Launch plan\n\nship it")
	item, err := SealItem(key, "notes/abc.md", 3, note)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(item, []byte("Launch plan")) {
		t.Fatal("the note shows in what the cloud keeps")
	}
	if got, err := OpenItem(key, "notes/abc.md", 3, item); err != nil || !bytes.Equal(got, note) {
		t.Fatalf("open: %q, %v", got, err)
	}
	for _, tc := range []struct {
		id      string
		version int64
	}{{"notes/other.md", 3}, {"notes/abc.md", 2}} {
		if _, err := OpenItem(key, tc.id, tc.version, item); !errors.Is(err, ErrOpen) {
			t.Errorf("opened as %s@%d: %v", tc.id, tc.version, err)
		}
	}
	other, _ := NewTeamKey()
	if _, err := OpenItem(other, "notes/abc.md", 3, item); !errors.Is(err, ErrOpen) {
		t.Fatalf("another team's key opened it: %v", err)
	}
}
