package server

import (
	"crypto/rand"
	"crypto/subtle"
	"net/http"
	"sync"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
)

// Setting a recovery key takes more than a device's token: a challenge
// sealed to the device's own key, opened. A stolen token, without the key
// that stays on the device, can't make the account's recovery key its own
// and have teammates seal the team's key to it.

const challengeLife = 5 * time.Minute

// challenges are the open recovery challenges, by device. Handlers on
// net/http's goroutines share them under mu.
type challenges struct {
	mu   sync.Mutex
	open map[string]challenge
}

type challenge struct {
	value   []byte
	expires time.Time
}

func (c *challenges) put(device string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.open == nil {
		c.open = map[string]challenge{}
	}
	now := time.Now()
	for d, ch := range c.open {
		if now.After(ch.expires) {
			delete(c.open, d)
		}
	}
	c.open[device] = challenge{value: value, expires: now.Add(challengeLife)}
}

// take reports whether proof is device's open challenge, which goes either
// way: one try each.
func (c *challenges) take(device string, proof []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch, ok := c.open[device]
	delete(c.open, device)
	return ok && time.Now().Before(ch.expires) && len(proof) == len(ch.value) && subtle.ConstantTimeCompare(proof, ch.value) == 1
}

func (s *Server) recoveryChallenge(w http.ResponseWriter, r *http.Request) {
	me := asker(r)
	value := make([]byte, seal.KeySize)
	rand.Read(value)
	sealed, err := seal.SealKey(value, me.Device.PublicKey)
	if err != nil {
		s.fail(w, http.StatusBadRequest, "this device has no key to seal a challenge to")
		return
	}
	s.recovery.put(me.Device.ID, value)
	s.reply(w, api.RecoveryChallenge{Sealed: sealed})
}
