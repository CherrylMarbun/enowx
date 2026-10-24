package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/enowx/enowx/proto"
)

// fanOut starts p's sessions, one per agent, each on a branch of its own,
// in one group. A session that can't start stops the rest; those started
// stay, listed.
func (s *Server) fanOut(p proto.FanOutParams) ([]proto.SessionInfo, error) {
	if len(p.Agents) < 2 {
		return nil, errors.New("fan-out: give two agents or more")
	}
	if p.BranchMode != proto.BranchNew || p.Branch == "" {
		return nil, errors.New("fan-out: each session needs a new branch: name it")
	}
	group := make([]byte, 6)
	_, _ = rand.Read(group)
	var out []proto.SessionInfo
	seen := map[string]int{}
	for _, ag := range p.Agents {
		seen[ag]++
		params := p.SessionCreateParams
		params.Agent, params.Group = ag, hex.EncodeToString(group)
		params.Branch = p.Branch + "-" + ag
		if n := seen[ag]; n > 1 {
			params.Branch += "-" + strconv.Itoa(n)
		}
		sess, err := s.createSession(params)
		if err != nil {
			return out, fmt.Errorf("fan-out: %s: %w", ag, err)
		}
		out = append(out, sess.Info())
	}
	return out, nil
}
