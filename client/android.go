package client

import "github.com/enowx/enowx/proto"

// Android devices on the daemon's host (docs/plans/mobile.md).

func (c *Client) AndroidStatus() (proto.AndroidStatus, error) {
	var st proto.AndroidStatus
	err := c.Call(proto.MethodAndroidStatus, nil, &st)
	return st, err
}

func (c *Client) AndroidSetup(session string) (proto.SessionInfo, error) {
	var tab proto.SessionInfo
	err := c.Call(proto.MethodAndroidSetup, proto.AndroidSetupParams{Session: session}, &tab)
	return tab, err
}

func (c *Client) AndroidStart(avd string, window bool) error {
	return c.Call(proto.MethodAndroidStart, proto.AndroidDeviceParams{AVD: avd, Window: window}, nil)
}

func (c *Client) AndroidStop(p proto.AndroidDeviceParams) error {
	return c.Call(proto.MethodAndroidStop, p, nil)
}

func (c *Client) AndroidConnect(p proto.AndroidConnectParams) error {
	return c.Call(proto.MethodAndroidConnect, p, nil)
}

func (c *Client) AndroidLogcat(p proto.AndroidLogcatParams) (proto.AndroidLog, error) {
	var l proto.AndroidLog
	err := c.Call(proto.MethodAndroidLogcat, p, &l)
	return l, err
}

func (c *Client) AndroidRun(p proto.AndroidRunParams) (proto.AndroidRun, error) {
	var r proto.AndroidRun
	err := c.Call(proto.MethodAndroidRun, p, &r)
	return r, err
}

func (c *Client) AndroidProject(dir string) (proto.AndroidProject, error) {
	var p proto.AndroidProject
	err := c.Call(proto.MethodAndroidProject, proto.AndroidProjectParams{Dir: dir}, &p)
	return p, err
}

// AndroidMirror shows device serial's screen: scrcpy's video stream
// (android.Mirror), until the tunnel is closed.
func (c *Client) AndroidMirror(p proto.AndroidMirrorParams) (*Tunnel, error) {
	return c.openTunnel(proto.MethodAndroidMirror, p)
}

// AndroidMirrorInput touches, presses a key or types on a screen shown.
func (c *Client) AndroidMirrorInput(p proto.AndroidMirrorInput) error {
	return c.Call(proto.MethodAndroidMirrorInput, p, nil)
}
