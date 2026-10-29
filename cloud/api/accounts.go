package api

import "time"

// Paths of a member's account and devices; a device's token, as a bearer
// token, says who asks.
const (
	PathMe      = "/v1/me"
	PathDevices = "/v1/devices"
	// PathDevice is one device, its ID last: DELETE revokes it.
	PathDevice  = "/v1/devices/{id}"
	PathSignOut = "/v1/signout"
)

// Kinds of devices.
const (
	DeviceApp  = "app"  // the app on a member's computer (or phone, later)
	DeviceHost = "host" // a host's daemon
)

// Account is a member of the cloud.
type Account struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email,omitempty"`
	Avatar string `json:"avatar,omitempty"`
	// GitHub is the GitHub login signed in with.
	GitHub  string    `json:"github,omitempty"`
	Created time.Time `json:"created"`
}

// Device is an app install or a host's daemon signed in to an account,
// with the public key the team's keys are sealed to (cloud/seal).
type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	PublicKey []byte    `json:"publicKey"`
	Created   time.Time `json:"created"`
	LastSeen  time.Time `json:"lastSeen"`
	// Current: the device asking.
	Current bool `json:"current,omitempty"`
	// Recovery: the account's recovery key, no device that signs in.
	Recovery bool `json:"recovery,omitempty"`
}

// Me is who asks: the account and the device whose token it is.
type Me struct {
	Account Account `json:"account"`
	Device  Device  `json:"device"`
}
