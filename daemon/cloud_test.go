package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A host signs in to a team cloud through its daemon: the address set, a
// code another of the account's devices approves, then the cloud asked as
// the host's device, its token kept on the host alone; signed out, the
// token works no more.
func TestHostSignsInToTheCloud(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	acc, _ := db.GitHubAccount(ctx, 1, "ada", "Ada", "", "")
	k, _ := seal.NewDeviceKey()
	_, macToken, _ := db.AddDevice(ctx, acc.ID, "Ada's Mac", api.DeviceApp, k.Public())
	mac, _ := cloudclient.New(cloud.URL, macToken)

	srv := startServer(t)
	changed := make(chan proto.CloudStatus, 8)
	c := connect(t, srv, func(e proto.Event) {
		if e.Name == proto.EventCloudChanged {
			var st proto.CloudStatus
			if decode(e.Data, &st) == nil {
				changed <- st
			}
		}
	})
	if st, err := c.CloudStatus(); err != nil || st.URL != "" || st.SignedIn {
		t.Fatalf("off at first: %+v %v", st, err)
	}
	if err := c.CloudCall("GET", api.PathMe, nil, nil); err == nil {
		t.Fatal("asked a cloud nobody set up")
	}
	if _, err := c.SetCloudURL("https://example.invalid"); err == nil {
		t.Fatal("an address that isn't a cloud was set")
	}
	if _, err := c.SetCloudURL(cloud.URL); err != nil {
		t.Fatal(err)
	}
	st, err := c.CloudSignIn(proto.CloudSignInParams{Name: "vps"})
	if err != nil || st.Pending == nil || st.Pending.UserCode == "" {
		t.Fatalf("sign in: %+v %v", st, err)
	}
	if err := mac.ApproveDevice(ctx, st.Pending.UserCode); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(15 * time.Second)
	for !st.SignedIn {
		select {
		case st = <-changed:
		case <-deadline:
			t.Fatalf("never signed in: %+v", st)
		}
	}
	if st.Account != "Ada" || st.DeviceName != "vps" || st.DeviceKind != proto.CloudDeviceHost || st.Pending != nil {
		t.Fatalf("signed in as: %+v", st)
	}
	var me api.Me
	if err := c.CloudCall("GET", api.PathMe, nil, &me); err != nil || me.Device.Name != "vps" {
		t.Fatalf("me through the daemon: %+v %v", me, err)
	}
	name := "enowX"
	var team api.Team
	if err := c.CloudCall("POST", api.PathTeams, api.TeamParams{Name: &name}, &team); err != nil || team.Role != api.RoleOwner {
		t.Fatalf("a team through the daemon: %+v %v", team, err)
	}
	var ce *client.CloudError
	if err := c.CloudCall("DELETE", "/v1/teams/nope", nil, nil); err == nil || !errors.As(err, &ce) || ce.Status != http.StatusNotFound {
		t.Fatalf("the cloud's error: %v", err)
	}
	// The token and key stay on the host, for its user alone.
	for _, f := range []string{"auth.json", "device.key"} {
		info, err := os.Stat(filepath.Join(srv.dataDir, "cloud", f))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s: %v %v", f, info, err)
		}
	}
	if data, _ := json.Marshal(st); strings.Contains(string(data), "enxc_") {
		t.Fatal("the token shows in the status")
	}

	if st, err = c.CloudSignOut(); err != nil || st.SignedIn {
		t.Fatalf("sign out: %+v %v", st, err)
	}
	if err := c.CloudCall("GET", api.PathMe, nil, nil); err == nil || !errors.As(err, &ce) || ce.Status != http.StatusUnauthorized {
		t.Fatalf("after signing out: %v", err)
	}
	if devices, _ := mac.Devices(ctx); len(devices) != 1 {
		t.Fatalf("the vps's device is still on the account: %+v", devices)
	}
	if st, err := c.SetCloudURL(""); err != nil || st.URL != "" {
		t.Fatalf("off again: %+v %v", st, err)
	}
}
