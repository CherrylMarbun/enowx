package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// A team's chat (docs/cloud.md): each message a file of the team's space,
// chat/<time>-<random>.json, synced and sealed with the rest of it, so the
// cloud holds ciphertext only. Sending writes it and syncs at once; a
// chat open pulls often (team.chat.refresh). A message that looks like it
// holds a secret isn't sent. Viewers read the chat.

const (
	chatLast = 100     // messages read when none is said
	chatMax  = 8 << 10 // a message's most bytes
)

func (s *Server) chatDir(team string) string { return filepath.Join(s.teamsDir(), team, "chat") }

// teamChat is team's last messages, oldest first.
func (s *Server) teamChat(p proto.TeamChatParams) ([]proto.ChatMessage, error) {
	if _, ok := s.spaceByTeam(p.Team); !ok {
		return nil, fmt.Errorf("spaces: team %s isn't on this host", p.Team)
	}
	entries, _ := os.ReadDir(s.chatDir(p.Team))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	last := p.Last
	if last <= 0 {
		last = chatLast
	}
	if len(names) > last {
		names = names[len(names)-last:]
	}
	out := []proto.ChatMessage{}
	for _, n := range names {
		if m, ok := s.readChat(p.Team, n); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Server) readChat(team, name string) (proto.ChatMessage, bool) {
	var m proto.ChatMessage
	data, err := os.ReadFile(filepath.Join(s.chatDir(team), name))
	if err != nil || json.Unmarshal(data, &m) != nil {
		return m, false
	}
	m.ID, m.Team = strings.TrimSuffix(name, ".json"), team
	return m, true
}

// sendChat sends a message to a team's chat, as the host's account.
func (s *Server) sendChat(p proto.ChatSendParams) (proto.ChatMessage, error) {
	sp, err := s.memberOf(p.Team)
	if err != nil {
		return proto.ChatMessage{}, errors.New("teams: viewers read the chat; members write in it")
	}
	text := strings.TrimSpace(p.Text)
	switch {
	case text == "":
		return proto.ChatMessage{}, errors.New("teams: say something")
	case len(text) > chatMax:
		return proto.ChatMessage{}, fmt.Errorf("teams: a message is %d KB at most", chatMax>>10)
	}
	if err := secretsIn(map[string][]byte{"the message": []byte(text)}); err != nil {
		return proto.ChatMessage{}, errors.New("teams: not sent, it looks like it holds a secret: keep credentials in the vault")
	}
	st, err := s.cloud.store.Get()
	if err != nil {
		return proto.ChatMessage{}, err
	}
	now := time.Now().UTC()
	var b [4]byte
	rand.Read(b[:])
	m := proto.ChatMessage{ID: fmt.Sprintf("%019d-%s", now.UnixNano(), hex.EncodeToString(b[:])), Team: sp.Team, At: now.Truncate(time.Millisecond),
		Account: st.AccountID, Name: st.Account, GitHub: st.GitHub, Text: text, Mentions: p.Mentions}
	data, _ := json.Marshal(m)
	if err := writeTeamFile(filepath.Join(s.chatDir(sp.Team), m.ID+".json"), data); err != nil {
		return proto.ChatMessage{}, err
	}
	s.broadcast(proto.EventTeamChat, []proto.ChatMessage{m})
	go s.syncSpace(sp.Team) // out now, not at the next sync
	return m, nil
}

// chatCame tells the apps the messages a sync brought.
func (s *Server) chatCame(team string, paths []string) {
	var list []proto.ChatMessage
	for _, p := range paths {
		if m, ok := s.readChat(team, filepath.Base(p)); ok {
			list = append(list, m)
		}
	}
	slices.SortFunc(list, func(a, b proto.ChatMessage) int { return strings.Compare(a.ID, b.ID) })
	if len(list) > 0 {
		s.broadcast(proto.EventTeamChat, list)
	}
}

var teamChatHandlers = map[string]handler{
	proto.MethodTeamChat: with(func(c *conn, p proto.TeamChatParams) (any, error) { return c.srv.teamChat(p) }),
	proto.MethodTeamChatSend: with(func(c *conn, p proto.ChatSendParams) (any, error) {
		return c.srv.sendChat(p)
	}),
	proto.MethodTeamChatRefresh: with(func(c *conn, p proto.SpaceRef) (any, error) {
		_, err := c.srv.pullSpace(p.Team)
		return ok, err
	}),
}
