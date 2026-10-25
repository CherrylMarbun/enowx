package userservice

import "testing"

// The user's daemon is enowxd.service; a daemon of another state folder
// has a unit of its own, the same each time, never the user's.
func TestUnitFor(t *testing.T) {
	if got := UnitFor("", "/home/ada"); got != Unit {
		t.Fatalf("the user's own: %s", got)
	}
	if got := UnitFor("/home/ada/.enowx-app/", "/home/ada"); got != Unit {
		t.Fatalf("ENOWX_HOME at the default: %s", got)
	}
	test := UnitFor("/tmp/enowx-synctest/home", "/home/ada")
	if test == Unit || test != UnitFor("/tmp/enowx-synctest/home", "/home/ada") || UnitFor("/tmp/other", "/home/ada") == test {
		t.Fatalf("another folder's: %s", test)
	}
}
