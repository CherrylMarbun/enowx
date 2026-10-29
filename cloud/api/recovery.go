package api

// A member's recovery key (docs/cloud.md, Keys): a key pair whose public
// half the cloud keeps as a device of theirs, with no token; the member
// keeps the private half. The team's keys are sealed to it like to any of
// their devices, so a new device of theirs, given the recovery key, opens
// those copies and gets the team's keys back when every other device is
// lost.
const (
	// PathRecovery sets the account's recovery key (POST RecoveryParams →
	// Device), in place of one before.
	PathRecovery = "/v1/recovery"
	// PathTeamRecovery gives the team's keys sealed to the account's
	// recovery key (GET → TeamKeys), and keeps a copy of one of them
	// sealed to the asking device (POST RecoverParams).
	PathTeamRecovery = "/v1/teams/{team}/recovery"
)

// RecoveryParams is a recovery key's public half.
type RecoveryParams struct {
	PublicKey []byte `json:"publicKey"`
}

// RecoverParams is an epoch's key, opened with the recovery key, sealed
// to the asking device.
type RecoverParams struct {
	Epoch  int    `json:"epoch"`
	Sealed []byte `json:"sealed"`
}
