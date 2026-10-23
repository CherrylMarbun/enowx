package client

import "github.com/enowx/enowx/proto"

// Transcript returns what changed in a session's conversation after
// revision since.
func (c *Client) Transcript(id string, since int64) (proto.Transcript, error) {
	return c.TranscriptPage(proto.TranscriptParams{ID: id, Since: since})
}

// TranscriptPage reads a part of session p.ID's conversation, as
// TranscriptParams says.
func (c *Client) TranscriptPage(p proto.TranscriptParams) (proto.Transcript, error) {
	var out proto.Transcript
	err := c.Call(proto.MethodSessionTranscript, p, &out)
	return out, err
}

// SwitchMode switches session id's agent to permission mode mode; the
// session's next transcript says the mode it reached.
func (c *Client) SwitchMode(id, mode string) error {
	return c.Call(proto.MethodSessionMode, proto.SessionModeParams{ID: id, Mode: mode}, nil)
}

// Commands lists session id's agent's slash commands; Loading says they
// are on the way: ask again.
func (c *Client) Commands(id string) (proto.AgentCommands, error) {
	var out proto.AgentCommands
	err := c.Call(proto.MethodSessionCommands, proto.SessionRef{ID: id}, &out)
	return out, err
}
