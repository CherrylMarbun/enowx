package client

import (
	"encoding/json"
	"fmt"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/proto"
)

// CloudStatus is the host's sign-in to a team cloud.
func (c *Client) CloudStatus() (proto.CloudStatus, error) {
	var st proto.CloudStatus
	err := c.Call(proto.MethodCloudStatus, nil, &st)
	return st, err
}

// SetCloudURL sets the host's cloud; "" turns it off.
func (c *Client) SetCloudURL(url string) (proto.CloudStatus, error) {
	var st proto.CloudStatus
	err := c.Call(proto.MethodCloudSetURL, proto.CloudSetURLParams{URL: url}, &st)
	return st, err
}

// CloudSignIn starts signing the host in; its Pending code waits for
// approval, and EventCloudChanged says when it is signed in.
func (c *Client) CloudSignIn(p proto.CloudSignInParams) (proto.CloudStatus, error) {
	var st proto.CloudStatus
	err := c.Call(proto.MethodCloudSignIn, p, &st)
	return st, err
}

// CloudCancel drops a sign-in waiting for approval.
func (c *Client) CloudCancel() (proto.CloudStatus, error) {
	var st proto.CloudStatus
	err := c.Call(proto.MethodCloudCancel, nil, &st)
	return st, err
}

// CloudSignOut signs the host out of its cloud.
func (c *Client) CloudSignOut() (proto.CloudStatus, error) {
	var st proto.CloudStatus
	err := c.Call(proto.MethodCloudSignOut, nil, &st)
	return st, err
}

// CloudError is what the cloud answered to a request that failed.
type CloudError struct {
	Status  int
	Message string
}

func (e *CloudError) Error() string { return "cloud: " + e.Message }

// CloudCall asks the cloud's API (cloud/api) through the host's daemon, as
// its device: in as the JSON body, the answer decoded into out.
func (c *Client) CloudCall(method, path string, in, out any) error {
	req := proto.CloudRequest{Method: method, Path: path}
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		req.Body = data
	}
	var res proto.CloudResponse
	if err := c.Call(proto.MethodCloudRequest, req, &res); err != nil {
		return err
	}
	if res.Status >= 300 {
		var e api.Error
		json.Unmarshal(res.Body, &e)
		if e.Error == "" {
			e.Error = fmt.Sprintf("the cloud answered %d", res.Status)
		}
		if res.Status == 401 {
			e.Error = "this host isn't signed in to the cloud (enowx-app cloud login)"
		}
		return &CloudError{Status: res.Status, Message: e.Error}
	}
	if out != nil && len(res.Body) > 0 {
		return json.Unmarshal(res.Body, out)
	}
	return nil
}

// NewRecoveryKey makes the account's recovery key, shown this once.
func (c *Client) NewRecoveryKey() (proto.CloudRecovery, error) {
	var out proto.CloudRecovery
	err := c.Call(proto.MethodCloudRecoveryNew, nil, &out)
	return out, err
}

// UseRecoveryKey gets the teams' keys back on the host with a recovery key.
func (c *Client) UseRecoveryKey(key string) (proto.CloudRecovery, error) {
	var out proto.CloudRecovery
	err := c.Call(proto.MethodCloudRecoveryUse, proto.CloudRecovery{Key: key}, &out)
	return out, err
}
