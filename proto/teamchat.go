package proto

import "time"

// A team's chat (docs/cloud.md): messages its members send each other,
// each kept in the team's space and synced with it, sealed with the
// team's key; the cloud holds ciphertext only. A message may mention the
// team's members, notes, canvases, projects, sessions and projects' files,
// answer another, and take members' reactions.
const (
	MethodTeamChat        = "team.chat"         // TeamChatParams → []ChatMessage, oldest first
	MethodTeamChatSend    = "team.chat.send"    // ChatSendParams → ChatMessage
	MethodTeamChatRefresh = "team.chat.refresh" // SpaceRef: takes what the team sent, now
	MethodTeamChatEdit    = "team.chat.edit"    // ChatEditParams → ChatMessage: its author's
	MethodTeamChatDelete  = "team.chat.delete"  // ChatRef: its author's, or an admin's
	MethodTeamChatReact   = "team.chat.react"   // ChatReactParams → ChatMessage: the user's reaction, added or taken back
)

// EventTeamChat carries the ChatMessages that came, were sent or were
// edited; EventTeamChatGone the ChatRefs of those deleted.
const (
	EventTeamChat     = "team.chat"
	EventTeamChatGone = "team.chat.gone"
)

// ChatRef names a message of Team's chat.
type ChatRef struct {
	Team string `json:"team"`
	ID   string `json:"id"`
}

// ChatEditParams writes Text and Mentions over message ID of Team's chat.
type ChatEditParams struct {
	Team     string        `json:"team"`
	ID       string        `json:"id"`
	Text     string        `json:"text"`
	Mentions []ChatMention `json:"mentions,omitempty"`
}

// ChatReactParams adds the user's reaction Emoji to message ID of Team's
// chat, or with Off takes it back.
type ChatReactParams struct {
	Team  string `json:"team"`
	ID    string `json:"id"`
	Emoji string `json:"emoji"`
	Off   bool   `json:"off,omitempty"`
}

// TeamChatParams reads Team's chat: its Last messages (100 when 0).
type TeamChatParams struct {
	Team string `json:"team"`
	Last int    `json:"last,omitempty"`
}

// ChatSendParams sends Text to Team's chat, mentioning items, in answer
// to message ReplyTo when set.
type ChatSendParams struct {
	Team     string        `json:"team"`
	Text     string        `json:"text"`
	Mentions []ChatMention `json:"mentions,omitempty"`
	ReplyTo  string        `json:"replyTo,omitempty"`
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
	Edited   *time.Time    `json:"edited,omitempty"`  // when its author last changed it
	ReplyTo  string        `json:"replyTo,omitempty"` // the message it answers
	// Reactions are the members' reactions, by emoji in the order they
	// first came; kept apart from the message, each member's their own.
	Reactions []ChatReaction `json:"reactions,omitempty"`
}

// ChatReaction is an emoji members reacted to a message with, by account.
type ChatReaction struct {
	Emoji    string   `json:"emoji"`
	Accounts []string `json:"accounts"`
}

// ChatMention is what a message names, as "@Title" in its text: a member,
// note, canvas, project, session or file (Kind), by ID (a member by their
// account, a project by its ref, a session by its host's device and ID,
// "device/id", a file by its path in Project, a project's ref), with its
// title as sent.
type ChatMention struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	Project string `json:"project,omitempty"`
}

// Kinds of ChatMention.
const (
	MentionNote    = "note"
	MentionCanvas  = "canvas"
	MentionProject = "project"
	MentionSession = "session"
	MentionUser    = "user"
	MentionFile    = "file"
)
