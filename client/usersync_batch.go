package client

import (
	"encoding/json"
	"fmt"

	"github.com/enowx/enowx/proto"
)

// Sync data is base64 in JSON. File counts alone do not bound a frame:
// a handful of canvas revisions can exceed the whole protocol budget.
func syncReadBatches(paths []string, files []proto.SyncEntry) ([][]string, error) {
	sizes := make(map[string]int64, len(files))
	for _, file := range files {
		sizes[file.Path] = file.Size
	}
	result, _ := json.Marshal(proto.SyncReadResult{Files: map[string][]byte{}})
	empty, _ := json.Marshal(proto.Response{ID: ^uint64(0), Result: result})
	var batches [][]string
	start, size := 0, len(empty)
	for i, path := range paths {
		dataSize := sizes[path]
		if dataSize < 0 || dataSize > proto.MaxPayload {
			return nil, fmt.Errorf("sync: %s exceeds the %d-byte frame limit", path, proto.MaxPayload)
		}
		key, _ := json.Marshal(path)
		// The colon, value's quotes and a comma; null also fits.
		entrySize := len(key) + 5 + int((dataSize+2)/3*4)
		if len(empty)+entrySize > proto.MaxPayload {
			return nil, fmt.Errorf("sync: %s exceeds the %d-byte frame limit", path, proto.MaxPayload)
		}
		if i > start && size+entrySize > proto.MaxPayload {
			batches = append(batches, paths[start:i])
			start, size = i, len(empty)
		}
		size += entrySize
	}
	if start < len(paths) {
		batches = append(batches, paths[start:])
	}
	return batches, nil
}

func syncWriteBatches(changes []proto.SyncChange, from string) ([][]proto.SyncChange, error) {
	params, _ := json.Marshal(proto.SyncWriteParams{Changes: []proto.SyncChange{}, From: from})
	empty, _ := json.Marshal(proto.Request{ID: ^uint64(0), Method: proto.MethodSyncWrite, Params: params})
	if len(empty) > proto.MaxPayload {
		return nil, fmt.Errorf("sync: write metadata exceeds the %d-byte frame limit", proto.MaxPayload)
	}
	var batches [][]proto.SyncChange
	start, size := 0, len(empty)
	for i, change := range changes {
		metadata := change
		metadata.Data = nil
		raw, _ := json.Marshal(metadata)
		entrySize := len(raw) + 1 // the comma between changes
		if len(change.Data) > 0 {
			entrySize += len(`,"data":""`) + (len(change.Data)+2)/3*4
		}
		if len(empty)+entrySize > proto.MaxPayload {
			return nil, fmt.Errorf("sync: %s exceeds the %d-byte frame limit", change.Path, proto.MaxPayload)
		}
		if i > start && size+entrySize > proto.MaxPayload {
			batches = append(batches, changes[start:i])
			start, size = i, len(empty)
		}
		size += entrySize
	}
	if start < len(changes) {
		batches = append(batches, changes[start:])
	}
	return batches, nil
}
