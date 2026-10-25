package client

import "github.com/enowx/enowx/proto"

// NotifySettings is how the apps notify the user of agents.
func (c *Client) NotifySettings() (proto.NotifySettings, error) {
	var st proto.NotifySettings
	err := c.Call(proto.MethodNotifyGet, nil, &st)
	return st, err
}

// SetNotifySettings keeps how the apps notify.
func (c *Client) SetNotifySettings(st proto.NotifySettings) (proto.NotifySettings, error) {
	var out proto.NotifySettings
	err := c.Call(proto.MethodNotifySet, st, &out)
	return out, err
}

// NotifySound asks the apps attached to play a sound.
func (c *Client) NotifySound(p proto.NotifySoundParams) error {
	return c.Call(proto.MethodNotifySound, p, nil)
}

// SaveNotifySound keeps a sound of the user's with the settings.
func (c *Client) SaveNotifySound(name string, data []byte) error {
	return c.Call(proto.MethodNotifySoundSave, proto.NotifySoundFile{Name: name, Data: data}, nil)
}

// NotifySoundData reads a sound: a built-in's name or "custom:FILE".
func (c *Client) NotifySoundData(name string) ([]byte, error) {
	var f proto.NotifySoundFile
	err := c.Call(proto.MethodNotifySoundRead, proto.NotifySoundFile{Name: name}, &f)
	return f.Data, err
}
