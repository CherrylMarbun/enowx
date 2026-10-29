package proto

import "time"

// A team's chat (docs/cloud.md): messages its members send each other,
// each kept in the team's space and synced with it, sealed with the
// team's key; the cloud holds ciphertext only. A message may mention the
// team's notes, canvases, projects and sessions.
const (
	MethodTeamChat        = "team.chat"         // TeamChatParams → []ChatMessage, oldest first
	MethodTeamChatSend    = "team.chat.send"    // ChatSendParams → ChatMessage
	MethodTeamChatRefresh = "team.chat.refresh" // SpaceRef: takes what the team sent, now
)

// EventTeamChat carries the ChatMessages that came or were sent.
const EventTeamChat = "team.chat"

// TeamChatParams reads Team's chat: its Last messages (100 when 0).
type TeamChatParams struct {
	Team string `json:"team"`
	Last int    `json:"last,omitempty"`
}

// ChatSendParams sends Text to Team's chat, mentioning items.
type ChatSendParams struct {
	Team     string        `json:"team"`
	Text     string        `json:"text"`
	Mentions []ChatMention `json:"mentions,omitempty"`
}

// ChatMessage is a message of a team's chat.
type ChatMessage struct {
	ID       string        `json:"id"`
	Team     string        `json:"team"`
	At       time.Time     `json:"at"`
	Account  string        `json:"account"`
	Name     string        `json:"name"`
	GitHub   string        `json:"github,omitempty"`
	Text     string        `json:"text"`
	Mentions []ChatMention `json:"mentions,omitempty"`
}

// ChatMention is an item a message names: a note, canvas, project or
// session (Kind), by ID (a project by its ref, a session by its host's
// device and ID, "device/id"), with its title as sent.
type ChatMention struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Kinds of ChatMention.
const (
	MentionNote    = "note"
	MentionCanvas  = "canvas"
	MentionProject = "project"
	MentionSession = "session"
)
