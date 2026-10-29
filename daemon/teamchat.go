package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/proto"
)

// A team's chat (docs/cloud.md): each message a file of the team's space,
// chat/<time>-<random>.json, synced and sealed with the rest of it, so the
// cloud holds ciphertext only. Sending writes it and syncs at once; a
// chat open pulls often (team.chat.refresh). A message that looks like it
// holds a secret isn't sent. Viewers read the chat. Its author edits a
// message; they or an admin delete it, the cloud then keeping no more of
// it than that it was deleted.

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
	text, err := chatText(p.Text)
	if err != nil {
		return proto.ChatMessage{}, err
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

// chatText is a message's text, trimmed, when it may be sent.
func chatText(text string) (string, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return "", errors.New("teams: say something")
	case len(text) > chatMax:
		return "", fmt.Errorf("teams: a message is %d KB at most", chatMax>>10)
	}
	if err := secretsIn(map[string][]byte{"the message": []byte(text)}); err != nil {
		return "", errors.New("teams: not sent, it looks like it holds a secret: keep credentials in the vault")
	}
	return text, nil
}

// chatID is how a message's file is named: its time and a random part.
var chatID = regexp.MustCompile(`^[0-9]{19}-[0-9a-f]{8}$`)

// ownChat is message ref, when the host's account may change it: its
// author's, or for delete an admin's.
func (s *Server) ownChat(ref proto.ChatRef, admins bool) (proto.ChatMessage, error) {
	sp, err := s.memberOf(ref.Team)
	if err != nil {
		return proto.ChatMessage{}, errors.New("teams: viewers read the chat; members write in it")
	}
	if !chatID.MatchString(ref.ID) {
		return proto.ChatMessage{}, fmt.Errorf("teams: no message %q", ref.ID)
	}
	m, ok := s.readChat(sp.Team, ref.ID+".json")
	if !ok {
		return proto.ChatMessage{}, fmt.Errorf("teams: no message %s in %s's chat", ref.ID, sp.Name)
	}
	st, err := s.cloud.store.Get()
	if err != nil {
		return proto.ChatMessage{}, err
	}
	admin := sp.Role == api.RoleOwner || sp.Role == api.RoleAdmin
	if m.Account != st.AccountID && !(admins && admin) {
		return proto.ChatMessage{}, errors.New("teams: only its author changes a message")
	}
	return m, nil
}

// editChat writes new text over the host's account's message.
func (s *Server) editChat(p proto.ChatEditParams) (proto.ChatMessage, error) {
	m, err := s.ownChat(proto.ChatRef{Team: p.Team, ID: p.ID}, false)
	if err != nil {
		return m, err
	}
	text, err := chatText(p.Text)
	if err != nil {
		return m, err
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	m.Text, m.Mentions, m.Edited = text, p.Mentions, &now
	data, _ := json.Marshal(m)
	if err := writeTeamFile(filepath.Join(s.chatDir(m.Team), m.ID+".json"), data); err != nil {
		return m, err
	}
	s.broadcast(proto.EventTeamChat, []proto.ChatMessage{m})
	go s.syncSpace(m.Team)
	return m, nil
}

// deleteChat deletes a message: its file goes, and the next sync tells
// the cloud, which drops its content.
func (s *Server) deleteChat(ref proto.ChatRef) error {
	m, err := s.ownChat(ref, true)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.chatDir(m.Team), m.ID+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.broadcast(proto.EventTeamChatGone, []proto.ChatRef{{Team: m.Team, ID: m.ID}})
	go s.syncSpace(m.Team)
	return nil
}

// chatGone tells the apps the messages a sync deleted.
func (s *Server) chatGone(team string, paths []string) {
	var refs []proto.ChatRef
	for _, p := range paths {
		refs = append(refs, proto.ChatRef{Team: team, ID: strings.TrimSuffix(path.Base(p), ".json")})
	}
	if len(refs) > 0 {
		s.broadcast(proto.EventTeamChatGone, refs)
	}
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
	proto.MethodTeamChatEdit: with(func(c *conn, p proto.ChatEditParams) (any, error) {
		return c.srv.editChat(p)
	}),
	proto.MethodTeamChatDelete: with(func(c *conn, p proto.ChatRef) (any, error) {
		return ok, c.srv.deleteChat(p)
	}),
}
