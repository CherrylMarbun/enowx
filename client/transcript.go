package client

import "github.com/enowx/enowx/proto"

// Transcript returns what changed in a session's conversation after
// revision since.
func (c *Client) Transcript(id string, since int64) (proto.Transcript, error) {
	var out proto.Transcript
	err := c.Call(proto.MethodSessionTranscript, proto.TranscriptParams{ID: id, Since: since}, &out)
	return out, err
}
