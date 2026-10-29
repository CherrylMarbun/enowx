package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/enowx/enowx/cloud/api"
)

// blobTime is a file's longest upload or download: one of 8 MB on a slow
// link takes longer than an API call's.
const blobTime = 3 * time.Minute

// slow is c, waiting as long as a file takes.
func (c *Client) slow() *Client {
	s := *c
	s.HTTP = &http.Client{Timeout: blobTime, Transport: c.HTTP.Transport}
	return &s
}

func blobPath(team, id string) string {
	return strings.NewReplacer("{team}", team, "{blob}", id).Replace(api.PathTeamBlob)
}

// PutBlob keeps sealed, file id of team.
func (c *Client) PutBlob(ctx context.Context, team, id string, sealed []byte) error {
	status, body, err := c.slow().Raw(ctx, "PUT", blobPath(team, id), sealed)
	return answerOf(status, body, err)
}

// Blob is file id of team, sealed.
func (c *Client) Blob(ctx context.Context, team, id string) ([]byte, error) {
	status, body, err := c.slow().Raw(ctx, "GET", blobPath(team, id), nil)
	if err := answerOf(status, body, err); err != nil {
		return nil, err
	}
	return body, nil
}

// DeleteBlob deletes file id of team.
func (c *Client) DeleteBlob(ctx context.Context, team, id string) error {
	status, body, err := c.Raw(ctx, "DELETE", blobPath(team, id), nil)
	return answerOf(status, body, err)
}

// answerOf is a raw answer's error: the cloud's, as call reads it.
func answerOf(status int, body []byte, err error) error {
	if err != nil || status < 300 {
		return err
	}
	var e api.Error
	json.Unmarshal(body, &e)
	if e.Error == "" {
		e.Error = "the cloud answered " + http.StatusText(status)
	}
	return &Error{Status: status, Message: e.Error}
}
