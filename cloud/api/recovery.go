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
	// PathRecoveryChallenge gives a RecoveryChallenge (GET).
	PathRecoveryChallenge = "/v1/recovery/challenge"
	// PathTeamRecovery gives the team's keys sealed to the account's
	// recovery key (GET → TeamKeys), and keeps a copy of one of them
	// sealed to the asking device (POST RecoverParams).
	PathTeamRecovery = "/v1/teams/{team}/recovery"
)

// RecoveryParams is a recovery key's public half.
type RecoveryParams struct {
	PublicKey []byte `json:"publicKey"`
	// Proof is the challenge PathRecoveryChallenge sealed to the asking
	// device, opened: a token alone, without the device's key, sets no
	// recovery key.
	Proof []byte `json:"proof"`
}

// RecoveryChallenge is a random value sealed to the asking device's key
// (seal.SealKey), to open for setting a recovery key, within minutes.
type RecoveryChallenge struct {
	Sealed []byte `json:"sealed"`
}

// RecoverParams is an epoch's key, opened with the recovery key, sealed
// to the asking device.
type RecoverParams struct {
	Epoch  int    `json:"epoch"`
	Sealed []byte `json:"sealed"`
}
