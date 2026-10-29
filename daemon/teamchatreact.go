package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/enowx/enowx/proto"
)

// Reactions to a team's chat messages: each member's to a message a file
// of their own, chat/reactions/<message>/<account>.json, synced and sealed
// as messages are, so only its member writes it (teamtrust.go) and two
// reacting at once never clash.

const (
	maxReactions = 20 // a member's emojis on one message
	maxEmoji     = 32 // an emoji's bytes: a sequence with modifiers and joiners
)

// storedReactions is a member's reactions to a message.
type storedReactions struct {
	Account string        `json:"account"`
	Emojis  []storedEmoji `json:"emojis"`
}

type storedEmoji struct {
	Emoji string    `json:"emoji"`
	At    time.Time `json:"at"`
}

func (s *Server) reactionsDir(team, id string) string {
	return filepath.Join(s.chatDir(team), "reactions", id)
}

// withReactions is message m with its members' reactions, by emoji in the
// order each first came.
func (s *Server) withReactions(m proto.ChatMessage) proto.ChatMessage {
	m.Reactions = nil
	entries, _ := os.ReadDir(s.reactionsDir(m.Team, m.ID))
	first := map[string]time.Time{}
	by := map[string][]string{}
	for _, e := range entries {
		var r storedReactions
		data, err := os.ReadFile(filepath.Join(s.reactionsDir(m.Team, m.ID), e.Name()))
		if err != nil || json.Unmarshal(data, &r) != nil || r.Account+".json" != e.Name() {
			continue
		}
		for _, x := range r.Emojis {
			if slices.Contains(by[x.Emoji], r.Account) {
				continue
			}
			by[x.Emoji] = append(by[x.Emoji], r.Account)
			if at, ok := first[x.Emoji]; !ok || x.At.Before(at) {
				first[x.Emoji] = x.At
			}
		}
	}
	for emoji, accounts := range by {
		slices.Sort(accounts)
		m.Reactions = append(m.Reactions, proto.ChatReaction{Emoji: emoji, Accounts: accounts})
	}
	slices.SortFunc(m.Reactions, func(a, b proto.ChatReaction) int {
		if c := first[a.Emoji].Compare(first[b.Emoji]); c != 0 {
			return c
		}
		return strings.Compare(a.Emoji, b.Emoji)
	})
	return m
}

// validEmoji says whether e may be a reaction: short, one line, no letters
// or digits, so it can't carry a message of its own.
func validEmoji(e string) bool {
	if e == "" || len(e) > maxEmoji {
		return false
	}
	for _, r := range e {
		if unicode.IsSpace(r) || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// reactChat adds the host's account's reaction to a message, or takes it
// back.
func (s *Server) reactChat(p proto.ChatReactParams) (proto.ChatMessage, error) {
	sp, err := s.memberOf(p.Team)
	if err != nil {
		return proto.ChatMessage{}, errors.New("teams: viewers read the chat; members react in it")
	}
	if !validEmoji(p.Emoji) {
		return proto.ChatMessage{}, errors.New("teams: a reaction is an emoji")
	}
	if !chatID.MatchString(p.ID) {
		return proto.ChatMessage{}, fmt.Errorf("teams: no message %q", p.ID)
	}
	m, ok := s.readChat(sp.Team, p.ID+".json")
	if !ok {
		return proto.ChatMessage{}, fmt.Errorf("teams: no message %s in %s's chat", p.ID, sp.Name)
	}
	st, err := s.cloud.store.Get()
	if err != nil {
		return m, err
	}
	file := filepath.Join(s.reactionsDir(m.Team, m.ID), st.AccountID+".json")
	r := storedReactions{Account: st.AccountID}
	if data, err := os.ReadFile(file); err == nil {
		json.Unmarshal(data, &r)
		r.Account = st.AccountID
	}
	has := slices.IndexFunc(r.Emojis, func(x storedEmoji) bool { return x.Emoji == p.Emoji })
	switch {
	case p.Off && has >= 0:
		r.Emojis = slices.Delete(r.Emojis, has, has+1)
	case !p.Off && has < 0:
		if len(r.Emojis) >= maxReactions {
			return m, fmt.Errorf("teams: %d reactions to a message at most", maxReactions)
		}
		r.Emojis = append(r.Emojis, storedEmoji{Emoji: p.Emoji, At: time.Now().UTC().Truncate(time.Millisecond)})
	default:
		return s.withReactions(m), nil
	}
	if len(r.Emojis) == 0 {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return m, err
		}
		removeEmptyDirs(filepath.Dir(file), s.chatDir(m.Team))
	} else {
		data, _ := json.Marshal(r)
		if err := writeTeamFile(file, data); err != nil {
			return m, err
		}
	}
	m = s.withReactions(m)
	s.broadcast(proto.EventTeamChat, []proto.ChatMessage{m})
	go s.syncSpace(m.Team)
	return m, nil
}

// reactionOf is the message and member a reaction's file p of a space
// is of, when p is one: chat/reactions/<message>/<account>.json.
func reactionOf(p string) (message, account string, ok bool) {
	parts := strings.Split(p, "/")
	if len(parts) != 4 || parts[0] != "chat" || parts[1] != "reactions" || !chatID.MatchString(parts[2]) || path.Ext(parts[3]) != ".json" {
		return "", "", false
	}
	return parts[2], strings.TrimSuffix(parts[3], ".json"), true
}
