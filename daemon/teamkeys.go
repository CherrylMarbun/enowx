package daemon

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"

	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/seal"
)

// A team's keys on this host (docs/cloud.md, Keys): its copies, sealed to
// the host's device key, opened when its space syncs and never kept open
// on disk. A copy is used only when it opens and matches its epoch's key
// check (epochs started before checks have none); one that doesn't is
// dropped, for a holder of the real key to seal again. A device holding
// keys seals every epoch's it holds for the members' devices that lack
// it; the first epoch nobody holds is started by an admin's device, which
// sends the key's check with it.

// errNoTeamKey is a sync waiting for a teammate's device to share the
// team's key with this one.
var errNoTeamKey = errors.New("waiting for the team's key: a teammate's device shares it when it next syncs")

// teamKeyring is the team's keys this host holds, by epoch, and the epoch
// new items are sealed with; with who the team's devices and members are,
// as the cloud said this sync.
type teamKeyring struct {
	keys    map[int][]byte
	current int
	devices []api.TeamDevice
	// roles are the members' roles by account, nil when the team
	// couldn't be read: then nothing is sealed for others, and files only
	// some may write aren't taken (teamwriters.go).
	roles map[string]string
}

// teamKeys opens the host's copies of team's keys; as an admin it starts
// the current epoch when no device holds it, and it shares the keys it
// holds with the members' devices that lack them.
func (s *Server) teamKeys(ctx context.Context, c *cloudclient.Client, team, role string) (teamKeyring, error) {
	dk, err := s.cloud.store.DeviceKey()
	if err != nil {
		return teamKeyring{}, err
	}
	st, err := s.cloud.store.Get()
	if err != nil {
		return teamKeyring{}, err
	}
	open := func() (teamKeyring, error) {
		held, err := c.TeamKeys(ctx, team)
		if err != nil {
			return teamKeyring{}, err
		}
		checks := map[int][]byte{}
		for _, ch := range held.Checks {
			checks[ch.Epoch] = ch.Check
		}
		kr := teamKeyring{keys: map[int][]byte{}, current: held.Epoch}
		for _, k := range held.Keys {
			key, err := dk.OpenKey(k.Sealed)
			check, checked := checks[k.Epoch]
			if err == nil && (!checked || seal.CheckKey(key, check)) {
				kr.keys[k.Epoch] = key
				continue
			}
			// Never used: a member may have sealed another key to this
			// device first. Dropped, the real one comes from a holder.
			s.logger().Warn("cloud: a copy of the team's key that doesn't open or isn't its epoch's: dropped, waiting for a teammate's device to seal the real one", "team", team, "epoch", k.Epoch)
			if err := c.DropKey(ctx, team, k.Epoch); err != nil {
				s.logger().Warn("cloud: drop a bad copy of the team's key", "team", team, "epoch", k.Epoch, "err", err)
			}
		}
		return kr, nil
	}
	kr, err := open()
	if err != nil {
		return teamKeyring{}, err
	}
	if kr.devices, err = c.TeamDevices(ctx, team); err != nil {
		return teamKeyring{}, err
	}
	switch t, err := c.Team(ctx, team); {
	case err == nil:
		kr.roles = map[string]string{}
		for _, m := range t.Members {
			kr.roles[m.Account.ID] = m.Role
		}
	case cloudclient.IsStatus(err, http.StatusNotFound):
		return teamKeyring{}, err // not a member any more
	default:
		s.logger().Warn("cloud: read the team's members", "team", team, "err", err)
	}
	// Who to seal for: the devices of accounts the team lists as members,
	// a viewer's host its own account's only (the cloud refuses more).
	sealFor := func(d api.TeamDevice) bool {
		if len(d.Device.PublicKey) == 0 || kr.roles == nil || kr.roles[d.Account] == "" {
			return false
		}
		return role != api.RoleViewer || d.Account == st.AccountID
	}
	if _, have := kr.keys[kr.current]; !have {
		nobody := !slices.ContainsFunc(kr.devices, func(d api.TeamDevice) bool { return slices.Contains(d.Epochs, kr.current) })
		if !nobody || (role != api.RoleOwner && role != api.RoleAdmin) || kr.roles == nil {
			return kr, errNoTeamKey
		}
		// The epoch's first key, for every member's device, this one too,
		// with its check.
		key, err := seal.NewTeamKey()
		if err != nil {
			return teamKeyring{}, err
		}
		var give []api.DeviceKey
		for _, d := range kr.devices {
			if !sealFor(d) {
				continue
			}
			if sealed, err := seal.SealKey(key, d.Device.PublicKey); err == nil {
				give = append(give, api.DeviceKey{Device: d.Device.ID, Sealed: sealed})
			}
		}
		if err := c.StartEpoch(ctx, team, kr.current, give, seal.KeyCheck(key)); err != nil {
			return kr, err
		}
		devices, roles := kr.devices, kr.roles
		if kr, err = open(); err != nil {
			return teamKeyring{}, err
		}
		kr.devices, kr.roles = devices, roles
		for i, d := range kr.devices {
			if slices.ContainsFunc(give, func(k api.DeviceKey) bool { return k.Device == d.Device.ID }) {
				kr.devices[i].Epochs = append(kr.devices[i].Epochs, kr.current)
			}
		}
	}
	// Every epoch's key held, for the devices lacking it: a new member's
	// device reads what was written before a removal too.
	for _, epoch := range slices.Sorted(maps.Keys(kr.keys)) {
		var give []api.DeviceKey
		for _, d := range kr.devices {
			if slices.Contains(d.Epochs, epoch) || !sealFor(d) {
				continue
			}
			if sealed, err := seal.SealKey(kr.keys[epoch], d.Device.PublicKey); err == nil {
				give = append(give, api.DeviceKey{Device: d.Device.ID, Sealed: sealed})
			}
		}
		if len(give) > 0 {
			if err := c.ShareKey(ctx, team, epoch, give); err != nil {
				s.logger().Warn("cloud: share the team's key", "team", team, "epoch", epoch, "err", err)
			}
		}
	}
	if _, have := kr.keys[kr.current]; !have {
		return kr, errNoTeamKey
	}
	return kr, nil
}
