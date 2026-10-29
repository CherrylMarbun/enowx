package proto

import (
	"encoding/json"
	"time"
)

// The cloud (docs/cloud.md): a host's daemon signs it in to a team cloud
// as a device of the user's account, and asks the cloud on its behalf.
// Nothing reaches a cloud while its address is unset.
const (
	MethodCloudStatus = "cloud.status" // → CloudStatus
	MethodCloudSetURL = "cloud.setURL" // CloudSetURLParams → CloudStatus; signs out of the one before
	MethodCloudSignIn = "cloud.signIn" // CloudSignInParams → CloudStatus, its Pending code to approve
	// MethodCloudCancel drops a sign-in waiting for its code's approval.
	MethodCloudCancel  = "cloud.cancel"  // → CloudStatus
	MethodCloudSignOut = "cloud.signOut" // → CloudStatus
	// MethodCloudRequest asks the cloud's API (cloud/api) as the host's
	// device: its token never leaves the daemon.
	MethodCloudRequest = "cloud.request" // CloudRequest → CloudResponse
)

// EventCloudChanged carries CloudStatus after it changed: signed in,
// out, a code to approve, or a sign-in that failed.
const EventCloudChanged = "cloud.changed"

// Kinds of devices a host signs in as.
const (
	CloudDeviceApp  = "app"  // the app's own computer
	CloudDeviceHost = "host" // a server's daemon
)

// CloudStatus is the host's sign-in to a cloud.
type CloudStatus struct {
	// URL is the cloud's address, "" while there is none: the cloud is off.
	URL      string `json:"url,omitempty"`
	SignedIn bool   `json:"signedIn,omitempty"`
	// Who: the account and the device this host is.
	AccountID  string `json:"accountId,omitempty"`
	Account    string `json:"account,omitempty"` // the account's name
	GitHub     string `json:"github,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	DeviceID   string `json:"deviceId,omitempty"`
	DeviceName string `json:"deviceName,omitempty"`
	DeviceKind string `json:"deviceKind,omitempty"`
	// Pending is a sign-in waiting for its code to be approved.
	Pending *CloudPending `json:"pending,omitempty"`
	// Err is why the last sign-in failed.
	Err string `json:"err,omitempty"`
}

// CloudPending is a code to approve, at VerifyURL (VerifyURLComplete has
// the code in it) or from a device signed in already.
type CloudPending struct {
	UserCode          string    `json:"userCode"`
	VerifyURL         string    `json:"verifyUrl,omitempty"`
	VerifyURLComplete string    `json:"verifyUrlComplete,omitempty"`
	Expires           time.Time `json:"expires"`
}

// CloudSetURLParams sets the cloud's address; "" turns the cloud off.
type CloudSetURLParams struct {
	URL string `json:"url"`
}

// CloudSignInParams signs the host in: as Kind (CloudDeviceApp or
// CloudDeviceHost), named Name (the host's name when empty).
type CloudSignInParams struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
}

// CloudRequest is a request to the cloud's API: a method, a path of it
// (/v1/…), and a JSON body.
type CloudRequest struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// CloudResponse is the cloud's answer: its status and JSON body.
type CloudResponse struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body,omitempty"`
}
