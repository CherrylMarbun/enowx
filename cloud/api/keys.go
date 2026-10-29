package api

// A team's key, per epoch, sealed to each of its members' devices
// (cloud/seal): the cloud keeps the sealed copies, never the key. A device
// holding an epoch's key seals it for the members' devices that lack it; an
// epoch with no key yet is started by an admin's device.
const (
	// PathTeamKeys gives the asking device its sealed copies.
	PathTeamKeys = "/v1/teams/{team}/keys"
	// PathTeamKey takes copies of an epoch's key sealed to devices (PUT),
	// and drops the asking device's own copy (DELETE), one that doesn't
	// open or match its epoch's check, to be sealed again.
	PathTeamKey = "/v1/teams/{team}/keys/{epoch}"
	// PathTeamDevices lists the members' devices, to seal the key for.
	PathTeamDevices = "/v1/teams/{team}/devices"
)

// SealedKeySize is a sealed team key's size: an ephemeral X25519 public
// key, the 32-byte key, and the AEAD's tag.
const SealedKeySize = 32 + 32 + 16

// SealedKey is a team key of an epoch, sealed to one device.
type SealedKey struct {
	Epoch  int    `json:"epoch"`
	Sealed []byte `json:"sealed"`
}

// TeamKeys is what the asking device holds of a team's keys, and the
// epoch new items are sealed with; Checks are the epochs' key checks, a
// copy's key to be compared with.
type TeamKeys struct {
	Epoch  int         `json:"epoch"`
	Keys   []SealedKey `json:"keys"`
	Checks []KeyCheck  `json:"checks,omitempty"`
}

// KeyCheck is an epoch's key check: a value only its key gives
// (seal.KeyCheck), set by the admin's device that started the epoch, so a
// device tells a copy of the real key from one a member sealed to it
// otherwise.
type KeyCheck struct {
	Epoch int    `json:"epoch"`
	Check []byte `json:"check"`
}

// TeamDevice is a member's device, and the epochs whose key it holds.
type TeamDevice struct {
	Account string `json:"account"`
	Name    string `json:"name"` // the member's
	Device  Device `json:"device"`
	Epochs  []int  `json:"epochs"`
}

// DeviceKey is an epoch's key sealed to device Device.
type DeviceKey struct {
	Device string `json:"device"`
	Sealed []byte `json:"sealed"`
}

// ShareKeyParams gives an epoch's key to devices; Check is its key check,
// given once, by the device starting the epoch.
type ShareKeyParams struct {
	Keys  []DeviceKey `json:"keys"`
	Check []byte      `json:"check,omitempty"`
}
