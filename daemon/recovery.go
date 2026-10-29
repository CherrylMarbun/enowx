package daemon

import (
	"context"
	"encoding/base32"
	"errors"
	"strings"

	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/proto"
)

// The account's recovery key (docs/cloud.md, Keys): a key pair made here,
// its public half set in the cloud as the account's recovery device, its
// private half shown to the user once and kept nowhere on the host. The
// hosts seal the teams' keys to it as to the account's other devices;
// given to a new device, it opens those copies and seals them to it.

const recoveryPrefix = "enxr-"

var recoveryEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// formatRecovery writes a private key as the user keeps it: in groups of
// four, to copy by hand.
func formatRecovery(priv []byte) string {
	enc := recoveryEncoding.EncodeToString(priv)
	var groups []string
	for len(enc) > 4 {
		groups = append(groups, enc[:4])
		enc = enc[4:]
	}
	return recoveryPrefix + strings.Join(append(groups, enc), "-")
}

func parseRecovery(key string) (*seal.DeviceKey, error) {
	k := strings.ToUpper(strings.NewReplacer("-", "", " ", "", "\n", "").Replace(strings.TrimSpace(key)))
	k = strings.TrimPrefix(k, strings.ToUpper(strings.ReplaceAll(recoveryPrefix, "-", "")))
	priv, err := recoveryEncoding.DecodeString(k)
	if err != nil || len(priv) != 32 {
		return nil, errors.New("cloud: that isn't a recovery key (enxr-XXXX-…)")
	}
	return seal.LoadDeviceKey(priv)
}

// newRecovery makes the account's recovery key, in place of the one
// before, and says it once.
func (s *Server) newRecovery() (proto.CloudRecovery, error) {
	c, err := s.cloudClient()
	if err != nil {
		return proto.CloudRecovery{}, err
	}
	k, err := seal.NewDeviceKey()
	if err != nil {
		return proto.CloudRecovery{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudTimeout)
	defer cancel()
	if _, err := c.SetRecovery(ctx, k.Public()); err != nil {
		return proto.CloudRecovery{}, err
	}
	s.logger().Info("cloud: a recovery key was made") // never the key
	s.teamSync.kick()                                 // the teams' keys sealed to it
	return proto.CloudRecovery{Key: formatRecovery(k.Private())}, nil
}

// useRecovery gets the teams' keys back on this host with a recovery key.
func (s *Server) useRecovery(key string) (proto.CloudRecovery, error) {
	rk, err := parseRecovery(key)
	if err != nil {
		return proto.CloudRecovery{}, err
	}
	c, err := s.cloudClient()
	if err != nil {
		return proto.CloudRecovery{}, err
	}
	mine, err := s.cloud.store.DeviceKey()
	if err != nil {
		return proto.CloudRecovery{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), teamSyncTime)
	defer cancel()
	out := proto.CloudRecovery{}
	var errs []error
	for _, sp := range s.spaces() {
		held, err := c.RecoveryKeys(ctx, sp.Team)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, k := range held.Keys {
			key, err := rk.OpenKey(k.Sealed)
			if err != nil {
				errs = append(errs, errors.New("cloud: the recovery key doesn't open "+sp.Name+"'s keys: is it the latest one made?"))
				break
			}
			sealed, err := seal.SealKey(key, mine.Public())
			if err != nil {
				return out, err
			}
			if err := c.Recover(ctx, sp.Team, k.Epoch, sealed); err != nil {
				errs = append(errs, err)
				continue
			}
			out.Recovered++
		}
	}
	s.teamSync.kick()
	if out.Recovered == 0 && len(errs) > 0 {
		return out, errors.Join(errs...)
	}
	return out, nil
}

// cloudClient is a client of the cloud as the host's device.
func (s *Server) cloudClient() (*cloudclient.Client, error) {
	st, err := s.cloud.store.Get()
	if err != nil {
		return nil, err
	}
	if !st.SignedIn() {
		return nil, errors.New("cloud: this host isn't signed in")
	}
	return cloudclient.New(st.URL, st.Token)
}
