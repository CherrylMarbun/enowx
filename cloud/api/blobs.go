package api

// A team's files: a chat's images, sealed by the sending device with the
// team's key, so the cloud keeps what it can't read. A member or more puts
// one (PUT, the sealed bytes as the body), any member gets it, its sender
// or an admin deletes it.
const (
	PathTeamBlob = "/v1/teams/{team}/blobs/{blob}"
)

// MaxBlob is the most a sealed file holds: an image of MaxImage and the
// seal's own bytes.
const (
	MaxImage = 8 << 20
	MaxBlob  = MaxImage + 1024
)
