package proto

// Notifications: the sounds the app plays when an agent finishes its
// turn, asks for input or fails, and how. The settings are the user's,
// kept in the synced settings/notify.json, so every host has the same.
const (
	MethodNotifyGet       = "notify.get"       // → NotifySettings
	MethodNotifySet       = "notify.set"       // NotifySettings → NotifySettings
	MethodNotifySound     = "notify.sound"     // NotifySoundParams: the apps attached play it
	MethodNotifySoundSave = "notify.soundSave" // NotifySoundFile: a sound of the user's, kept with the settings
	MethodNotifySoundRead = "notify.soundRead" // NotifySoundFile (its Name) → NotifySoundFile
)

// Notify events.
const (
	// EventNotifySound asks the apps to play a sound (NotifySoundParams).
	EventNotifySound = "notify.sound"
	// EventNotifySettings carries NotifySettings after they changed.
	EventNotifySettings = "notify.settings"
)

// Kinds of notification sound.
const (
	NotifyDone  = "done"  // an agent finished its turn
	NotifyInput = "input" // an agent asks for input or permission
	NotifyChat  = "chat"  // a team member wrote in a team's chat
	NotifyError = "error" // an agent failed or crashed
)

// NotifySettings is how the app notifies: Off silences every sound;
// Volume is from 0 to 1, apart from the system's; Sounds names the sound
// of each kind (a built-in's name, "custom:FILE", or "none").
type NotifySettings struct {
	Off    bool              `json:"off,omitempty"`
	Volume float64           `json:"volume"`
	Sounds map[string]string `json:"sounds,omitempty"`
	// Quiet silences sounds from QuietFrom to QuietTo ("22:00",
	// "07:00"), which may span midnight.
	Quiet     bool   `json:"quiet,omitempty"`
	QuietFrom string `json:"quietFrom,omitempty"`
	QuietTo   string `json:"quietTo,omitempty"`
	// Watched also sounds for the session the user is looking at.
	Watched bool `json:"watched,omitempty"`
	// NoBanner leaves out the desktop notification shown with a sound;
	// Badge counts the sessions waiting on the app's icon.
	NoBanner bool `json:"noBanner,omitempty"`
	Badge    bool `json:"badge,omitempty"`
	// MutedAgents and MutedSessions are silenced, by agent ID and by
	// session ID.
	MutedAgents   []string `json:"mutedAgents,omitempty"`
	MutedSessions []string `json:"mutedSessions,omitempty"`
	// Chat is which team chat messages notify: "" all of them,
	// "mentions" those naming the user, "off" none.
	Chat string `json:"chat,omitempty"`
	// ChatTeams sets Chat per team, by team ID, over Chat. Mentions of the
	// user and replies to them notify whatever these say.
	ChatTeams map[string]string `json:"chatTeams,omitempty"`
}

// NotifySoundParams asks for a sound of Kind, for Session if any; Text is
// what a banner says.
type NotifySoundParams struct {
	Kind    string `json:"kind"`
	Session string `json:"session,omitempty"`
	Text    string `json:"text,omitempty"`
}

// NotifySoundFile is a sound file of the user's: a WAV or an MP3.
type NotifySoundFile struct {
	Name string `json:"name"`
	Data []byte `json:"data,omitempty"`
}
