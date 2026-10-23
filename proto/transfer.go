package proto

// Transfers copy a project and its sessions from one host to another
// (docs/transfer.md): the source's daemon packs them into an archive, the
// app reads it in parts and writes them to the target's daemon, which
// unpacks it into a folder and starts the sessions again.
const (
	MethodTransferPack    = "transfer.pack"    // TransferPackParams → TransferInfo
	MethodTransferRead    = "transfer.read"    // TransferReadParams → TransferPart
	MethodTransferWrite   = "transfer.write"   // TransferPart
	MethodTransferUnpack  = "transfer.unpack"  // TransferUnpackParams → TransferResult
	MethodTransferDiscard = "transfer.discard" // TransferRef
)

// TransferPart is how much of an archive one read or write carries.
const TransferPartSize = 1 << 20

// TransferPackParams packs Project's folder (as git keeps it, or without
// its dependencies), with the .env files only when Env says so, and the
// sessions named in Sessions, all of them when it is nil.
type TransferPackParams struct {
	Project  string   `json:"project"`
	Sessions []string `json:"sessions"` // null: all; []: none
	Env      bool     `json:"env,omitempty"`
}

// TransferInfo is an archive packed: its id, its size in bytes, and what
// it carries.
type TransferInfo struct {
	ID       string `json:"id"`
	Size     int64  `json:"size"`
	Files    int    `json:"files"`
	Sessions int    `json:"sessions"`
}

// TransferReadParams reads Size bytes of archive ID from Offset.
type TransferReadParams struct {
	ID     string `json:"id"`
	Offset int64  `json:"offset"`
	Size   int    `json:"size"`
}

// TransferPart is a part of an archive, read or written at Offset; a
// write at offset 0 starts the archive anew.
type TransferPart struct {
	ID     string `json:"id"`
	Offset int64  `json:"offset"`
	Data   []byte `json:"data"`
}

// TransferUnpackParams unpacks archive ID into the folder Dest ("~/…"
// for the home folder), which must not hold anything yet.
type TransferUnpackParams struct {
	ID   string `json:"id"`
	Dest string `json:"dest"`
}

// TransferResult is what an archive became: the project, its sessions
// started again, and notes on what couldn't come along.
type TransferResult struct {
	Project  ProjectInfo   `json:"project"`
	Sessions []SessionInfo `json:"sessions"`
	Notes    []string      `json:"notes,omitempty"`
}

// TransferRef names an archive, to delete it.
type TransferRef struct {
	ID string `json:"id"`
}
