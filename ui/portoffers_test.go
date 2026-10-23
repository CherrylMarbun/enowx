package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

func TestPortOffersFollowThePorts(t *testing.T) {
	a := &App{}
	vite := proto.Port{Port: 5173, Process: "node", PID: 1}
	api := proto.Port{Port: 8080, Process: "go", PID: 2}
	a.portsChanged(proto.PortsChange{Session: "s1", Ports: []proto.Port{vite}, Opened: []proto.Port{vite}})
	a.portsChanged(proto.PortsChange{Session: "s1", Ports: []proto.Port{vite, api}, Opened: []proto.Port{api}})
	if len(a.portOffers) != 2 {
		t.Fatalf("offers = %+v", a.portOffers)
	}
	// The dev server stops: its offer goes.
	a.portsChanged(proto.PortsChange{Session: "s1", Ports: []proto.Port{api}})
	if len(a.portOffers) != 1 || a.portOffers[0].port.Port != 8080 {
		t.Fatalf("after 5173 closed: %+v", a.portOffers)
	}
	a.dropPortOffer(8080)
	if len(a.portOffers) != 0 {
		t.Fatal("a dismissed offer stays")
	}
}

func TestPortOfferCardOffersToOpen(t *testing.T) {
	a := &App{client: &client.Client{}}
	a.sessions = []proto.SessionInfo{{ID: "s1", Name: "search"}}
	a.portsChanged(proto.PortsChange{Session: "s1", Opened: []proto.Port{{Port: 5173, Process: "node"}}, Ports: []proto.Port{{Port: 5173}}})
	tt := ui.NewTester(func(c *ui.Context) { a.portOfferCards(c) }, 800, 500)
	tt.Frame()
	for _, want := range []string{"node is listening on :5173", "in search", "Open", "Ports"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("Dismiss"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("node is listening on :5173") {
		t.Fatal("dismissed, still shown")
	}
}
