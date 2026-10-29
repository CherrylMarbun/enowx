package daemon

import (
	"context"
	"errors"
	"slices"

	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/seal"
)

// A team's keys on this host (docs/cloud.md, Keys): its copies, sealed to
// the host's device key, opened when its space syncs and never kept open
// on disk. A device holding the current epoch's key seals it for the
// members' devices that lack it; the first epoch nobody holds is started
// by an admin's device.

// errNoTeamKey is a sync waiting for a teammate's device to share the
// team's key with this one.
var errNoTeamKey = errors.New("waiting for the team's key: a teammate's device shares it when it next syncs")

// teamKeyring is the team's keys this host holds, by epoch, and the epoch
// new items are sealed with.
type teamKeyring struct {
	keys    map[int][]byte
	current int
}

// teamKeys opens the host's copies of team's keys; as an admin it starts
// the current epoch when no device holds it, and it shares the current
// key with the members' devices that lack it.
func (s *Server) teamKeys(ctx context.Context, c *cloudclient.Client, team, role string) (teamKeyring, error) {
	dk, err := s.cloud.store.DeviceKey()
	if err != nil {
		return teamKeyring{}, err
	}
	open := func() (teamKeyring, error) {
		held, err := c.TeamKeys(ctx, team)
		if err != nil {
			return teamKeyring{}, err
		}
		kr := teamKeyring{keys: map[int][]byte{}, current: held.Epoch}
		for _, k := range held.Keys {
			if key, err := dk.OpenKey(k.Sealed); err == nil {
				kr.keys[k.Epoch] = key
			}
		}
		return kr, nil
	}
	kr, err := open()
	if err != nil {
		return teamKeyring{}, err
	}
	devices, err := c.TeamDevices(ctx, team)
	if err != nil {
		return teamKeyring{}, err
	}
	key, have := kr.keys[kr.current]
	if !have {
		nobody := !slices.ContainsFunc(devices, func(d api.TeamDevice) bool { return slices.Contains(d.Epochs, kr.current) })
		if !nobody || (role != api.RoleOwner && role != api.RoleAdmin) {
			return kr, errNoTeamKey
		}
		// The epoch's first key, for every member's device, this one too.
		if key, err = seal.NewTeamKey(); err != nil {
			return teamKeyring{}, err
		}
	}
	var give []api.DeviceKey
	for _, d := range devices {
		if (have && slices.Contains(d.Epochs, kr.current)) || len(d.Device.PublicKey) == 0 {
			continue
		}
		sealed, err := seal.SealKey(key, d.Device.PublicKey)
		if err != nil {
			continue
		}
		give = append(give, api.DeviceKey{Device: d.Device.ID, Sealed: sealed})
	}
	if len(give) > 0 {
		if err := c.ShareKey(ctx, team, kr.current, give); err != nil {
			if !have {
				return kr, err
			}
			s.logger().Warn("cloud: share the team's key", "team", team, "err", err)
		}
	}
	if !have {
		return open()
	}
	return kr, nil
}
