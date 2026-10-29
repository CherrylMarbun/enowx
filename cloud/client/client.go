// Package client talks to a cloud server (cloud/server) for a daemon: its
// API (cloud/api) over HTTPS, with the device's token once signed in.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enowx/enowx/cloud/api"
)

// Client is a cloud's API at URL, as the device whose Token it is (empty:
// not signed in).
type Client struct {
	URL   string
	Token string
	HTTP  *http.Client
}

// New is a client of the cloud at rawURL, signed in with token if any.
func New(rawURL, token string) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("cloud: %q isn't a cloud's address (https://…)", rawURL)
	}
	return &Client{URL: strings.TrimSuffix(u.String(), "/"), Token: token, HTTP: &http.Client{Timeout: 20 * time.Second}}, nil
}

// Error is what the cloud said of a request that failed.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return "cloud: " + e.Message }

// IsStatus says err is the cloud answering status.
func IsStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

// call sends in as JSON to path and decodes the answer into out.
func (c *Client) call(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cloud: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		var e api.Error
		json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&e)
		if e.Error == "" {
			e.Error = res.Status
		}
		return &Error{Status: res.StatusCode, Message: e.Error}
	}
	if out == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(out)
}

// path is a path of the API with its {names} filled, escaped.
func path(p string, kv ...string) string {
	for i := 0; i+1 < len(kv); i += 2 {
		p = strings.Replace(p, "{"+kv[i]+"}", url.PathEscape(kv[i+1]), 1)
	}
	return p
}

// Info is what the server is and can do.
func (c *Client) Info(ctx context.Context) (api.Info, error) {
	var in api.Info
	err := c.call(ctx, "GET", api.PathInfo, nil, &in)
	if err == nil && in.Name != "enowx-cloud" {
		err = fmt.Errorf("cloud: %s isn't an enowX cloud", c.URL)
	}
	return in, err
}

// StartSignIn asks for a device code to sign this device in with.
func (c *Client) StartSignIn(ctx context.Context, p api.DeviceCodeParams) (api.DeviceCode, error) {
	var dc api.DeviceCode
	err := c.call(ctx, "POST", api.PathDeviceCode, p, &dc)
	return dc, err
}

// ErrPending is SignInToken's error while the code waits for approval.
var ErrPending = errors.New("cloud: waiting for the code's approval")

// SignInToken asks once for the token of device code dc: ErrPending while
// it isn't approved.
func (c *Client) SignInToken(ctx context.Context, deviceCode string) (api.SignedIn, error) {
	var in api.SignedIn
	err := c.call(ctx, "POST", api.PathDeviceToken, api.DeviceTokenParams{DeviceCode: deviceCode}, &in)
	var e *Error
	if errors.As(err, &e) {
		switch e.Message {
		case api.ErrPending:
			return in, ErrPending
		case api.ErrExpired:
			return in, errors.New("cloud: the code expired before it was approved: sign in again")
		}
	}
	return in, err
}

// WaitSignIn asks for dc's token every interval until it is approved, it
// expires, or ctx ends.
func (c *Client) WaitSignIn(ctx context.Context, dc api.DeviceCode) (api.SignedIn, error) {
	every := time.Duration(max(dc.Interval, 1)) * time.Second
	for {
		in, err := c.SignInToken(ctx, dc.DeviceCode)
		if !errors.Is(err, ErrPending) {
			return in, err
		}
		select {
		case <-ctx.Done():
			return in, ctx.Err()
		case <-time.After(every):
		}
	}
}

// Me is who the token is.
func (c *Client) Me(ctx context.Context) (api.Me, error) {
	var me api.Me
	err := c.call(ctx, "GET", api.PathMe, nil, &me)
	return me, err
}

// Devices are the account's devices.
func (c *Client) Devices(ctx context.Context) ([]api.Device, error) {
	var list []api.Device
	err := c.call(ctx, "GET", api.PathDevices, nil, &list)
	return list, err
}

// RemoveDevice signs one of the account's devices out.
func (c *Client) RemoveDevice(ctx context.Context, id string) error {
	return c.call(ctx, "DELETE", path(api.PathDevice, "id", id), nil, nil)
}

// SignOut signs this device out: its token stops working.
func (c *Client) SignOut(ctx context.Context) error {
	return c.call(ctx, "POST", api.PathSignOut, nil, nil)
}

// ApproveDevice signs the device showing userCode in to this account.
func (c *Client) ApproveDevice(ctx context.Context, userCode string) error {
	return c.call(ctx, "POST", api.PathDeviceApprove, api.DeviceApproveParams{UserCode: userCode}, nil)
}

// Teams are the account's teams.
func (c *Client) Teams(ctx context.Context) ([]api.Team, error) {
	var list []api.Team
	err := c.call(ctx, "GET", api.PathTeams, nil, &list)
	return list, err
}

// Team is a team with its members.
func (c *Client) Team(ctx context.Context, id string) (api.Team, error) {
	var t api.Team
	err := c.call(ctx, "GET", path(api.PathTeam, "team", id), nil, &t)
	return t, err
}

// CreateTeam makes a team, this account its owner.
func (c *Client) CreateTeam(ctx context.Context, name string) (api.Team, error) {
	var t api.Team
	err := c.call(ctx, "POST", api.PathTeams, api.TeamParams{Name: &name}, &t)
	return t, err
}

// UpdateTeam renames a team or changes its rules.
func (c *Client) UpdateTeam(ctx context.Context, id string, p api.TeamParams) error {
	return c.call(ctx, "PATCH", path(api.PathTeam, "team", id), p, nil)
}

// DeleteTeam deletes a team.
func (c *Client) DeleteTeam(ctx context.Context, id string) error {
	return c.call(ctx, "DELETE", path(api.PathTeam, "team", id), nil, nil)
}

// SetRole changes a member's role.
func (c *Client) SetRole(ctx context.Context, team, account, role string) error {
	return c.call(ctx, "PATCH", path(api.PathMember, "team", team, "account", account), api.RoleParams{Role: role}, nil)
}

// RemoveMember removes a member, or leaves when it is this account.
func (c *Client) RemoveMember(ctx context.Context, team, account string) error {
	return c.call(ctx, "DELETE", path(api.PathMember, "team", team, "account", account), nil, nil)
}

// Invite invites to a team.
func (c *Client) Invite(ctx context.Context, team string, p api.InviteParams) (api.Invite, error) {
	var in api.Invite
	err := c.call(ctx, "POST", path(api.PathInvites, "team", team), p, &in)
	return in, err
}

// Invites are a team's open invites.
func (c *Client) Invites(ctx context.Context, team string) ([]api.Invite, error) {
	var list []api.Invite
	err := c.call(ctx, "GET", path(api.PathInvites, "team", team), nil, &list)
	return list, err
}

// DeleteInvite withdraws an invite.
func (c *Client) DeleteInvite(ctx context.Context, team, id string) error {
	return c.call(ctx, "DELETE", path(api.PathInvite, "team", team, "invite", id), nil, nil)
}

// Join accepts an invite: its token or link.
func (c *Client) Join(ctx context.Context, token string) (api.Team, error) {
	var t api.Team
	err := c.call(ctx, "POST", api.PathJoin, api.JoinParams{Token: token}, &t)
	return t, err
}
