package api

import "time"

// Signing a device in with a code, for a host's daemon or the command line,
// which have no browser of their own: the device asks for a code
// (PathDeviceCode), shows its user code, and asks for its token
// (PathDeviceToken) every Interval while a member signed in elsewhere looks
// at what asks (PathDevicePending) and approves it (PathDeviceApprove).
const (
	PathDeviceCode    = "/v1/auth/device"
	PathDeviceToken   = "/v1/auth/device/token"
	PathDevicePending = "/v1/auth/device/pending/{code}"
	PathDeviceApprove = "/v1/auth/device/approve"
)

// Errors PathDeviceToken answers with (Error.Error), status 400, while
// there is no token to give.
const (
	ErrPending = "authorization_pending" // not approved yet: ask again
	ErrExpired = "expired_token"         // expired, used or unknown: start over
)

// DeviceCodeParams is the device asking: what it is and its public key
// (cloud/seal), as AddDevice takes them.
type DeviceCodeParams struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	PublicKey []byte `json:"publicKey"`
}

// DeviceCode is what a device gets to sign in with: DeviceCode it keeps and
// asks its token with, UserCode it shows, to approve at VerifyURL.
type DeviceCode struct {
	DeviceCode string `json:"deviceCode"`
	UserCode   string `json:"userCode"`
	VerifyURL  string `json:"verifyUrl,omitempty"`
	// VerifyURLComplete is VerifyURL with the code in it, to open.
	VerifyURLComplete string    `json:"verifyUrlComplete,omitempty"`
	Interval          int       `json:"interval"` // seconds between asks
	Expires           time.Time `json:"expires"`
}

// DeviceTokenParams asks for the token of a device code approved.
type DeviceTokenParams struct {
	DeviceCode string `json:"deviceCode"`
}

// SignedIn is a device signed in: its token, to keep, and who it is.
type SignedIn struct {
	Token string `json:"token"`
	Me    Me     `json:"me"`
}

// PendingDevice is a device waiting for its code's approval, for the member
// to see before approving.
type PendingDevice struct {
	Name    string    `json:"name"`
	Kind    string    `json:"kind"`
	Created time.Time `json:"created"`
	// From is the address the sign-in started from, Place its country as
	// the cloud's proxy tells it; SameNetwork says it is the asker's own:
	// a sign-in from elsewhere, maybe someone else's, stands out.
	From        string `json:"from,omitempty"`
	Place       string `json:"place,omitempty"`
	SameNetwork bool   `json:"sameNetwork,omitempty"`
}

// DeviceApproveParams approves the device that shows UserCode.
type DeviceApproveParams struct {
	UserCode string `json:"userCode"`
}
