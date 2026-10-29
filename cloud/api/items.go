package api

import "time"

// A team's items: the files of its space (notes, canvases, their images),
// each sealed on a member's device with the team's key of an epoch
// (cloud/seal). The cloud keeps an item's kind, size, version and when
// it changed; its path and content are in the sealed part. Every change
// takes the team's next sequence number, so a device asks for what
// changed after the last it saw.
const (
	// PathItems lists the items changed after ?after= (a Seq), oldest
	// first, at most a page of them.
	PathItems = "/v1/teams/{team}/items"
	// PathItem writes one (PUT): its next version, or its deletion.
	PathItem = "/v1/teams/{team}/items/{id}"
)

// MaxItemSize is the most a sealed item holds.
const MaxItemSize = 16 << 20

// Kinds of items, as the cloud sees them.
const (
	ItemNote   = "note"
	ItemCanvas = "canvas"
	ItemFile   = "file"
	ItemChat   = "chat" // a message of the team's chat
)

// Item is a team's item at its latest version.
type Item struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Version int64  `json:"version"`
	Epoch   int    `json:"epoch"`
	// Sealed is its content, none once deleted.
	Sealed    []byte    `json:"sealed,omitempty"`
	Deleted   bool      `json:"deleted,omitempty"`
	Seq       int64     `json:"seq"`
	Updated   time.Time `json:"updated"`
	UpdatedBy string    `json:"updatedBy"` // the device's ID
}

// Items is a page of items changed after a Seq, and the Seq to ask after
// next; More says another page waits.
type Items struct {
	Items []Item `json:"items"`
	Seq   int64  `json:"seq"`
	More  bool   `json:"more,omitempty"`
}

// PutItemParams writes an item: Version is the one it becomes, one after
// the version it was changed from (1 for a new one); Epoch the key's it is
// sealed with, the team's current. A version taken already is refused with
// 409 and the item as it is.
type PutItemParams struct {
	Kind    string `json:"kind"`
	Version int64  `json:"version"`
	Epoch   int    `json:"epoch"`
	Sealed  []byte `json:"sealed,omitempty"`
	Deleted bool   `json:"deleted,omitempty"`
}
