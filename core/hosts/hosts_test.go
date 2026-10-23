package hosts

import (
	"path/filepath"
	"testing"
)

// A list exported from one device imports on another, without what it
// already has.
func TestExportImport(t *testing.T) {
	a, err := Open(filepath.Join(t.TempDir(), "hosts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Add(Host{Name: "vps", Address: "root@1.2.3.4", PasswordAuth: true, Jump: "bastion"}); err != nil {
		t.Fatal(err)
	}
	data, err := a.Export()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Open(filepath.Join(t.TempDir(), "hosts.json"))
	if _, err := b.Add(Host{Name: "same", Address: "root@1.2.3.4"}); err != nil {
		t.Fatal(err)
	}
	if n, err := b.Import(data); err != nil || n != 0 {
		t.Fatalf("imported %d, %v: the address was there", n, err)
	}
	c, _ := Open(filepath.Join(t.TempDir(), "hosts.json"))
	if n, err := c.Import(data); err != nil || n != 1 {
		t.Fatalf("imported %d, %v", n, err)
	}
	if h := c.List()[0]; h.Jump != "bastion" || !h.PasswordAuth || h.ID == "" {
		t.Fatalf("imported %+v", h)
	}
}
