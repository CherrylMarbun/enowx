package envsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
)

// EditJSON changes JSON (with comments, as agents' settings allow) at
// path, keeping the rest and its order: sets value there, making the
// objects on the way, or with nil removes it. data nil is an empty
// object. Comments are dropped; hadComments says there were.
func EditJSON(data []byte, path []string, value json.RawMessage) (out []byte, hadComments bool, err error) {
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	data, hadComments = stripJSONC(data)
	edited, err := editMembers(data, path, value)
	if err != nil {
		return nil, hadComments, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, edited, "", "  "); err != nil {
		return nil, hadComments, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), hadComments, nil
}

func editMembers(data []byte, path []string, value json.RawMessage) ([]byte, error) {
	ms, err := objectMembers(data)
	if err != nil {
		return nil, err
	}
	key := path[0]
	if len(path) == 1 {
		if value == nil {
			kept := ms[:0]
			for _, m := range ms {
				if m.key != key {
					kept = append(kept, m)
				}
			}
			return encodeObject(kept), nil
		}
		return encodeObject(setMember(ms, key, value)), nil
	}
	inner := []byte("{}")
	for _, m := range ms {
		if m.key == key {
			inner = m.value
		}
	}
	if !bytes.HasPrefix(bytes.TrimSpace(inner), []byte("{")) {
		if value == nil {
			return encodeObject(ms), nil
		}
		return nil, errors.New(key + " isn't an object")
	}
	edited, err := editMembers(inner, path[1:], value)
	if err != nil {
		return nil, err
	}
	if value == nil && bytes.Equal(edited, []byte("{}")) {
		return editMembers(data, path[:1], nil) // nothing left in it
	}
	return encodeObject(setMember(ms, key, edited)), nil
}

// ReadFile is a file's content, nil when it isn't there.
func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// WriteFileAtomic writes a file whole or not at all, making its folder.
func WriteFileAtomic(path string, data []byte, mode fs.FileMode) error {
	return writeFileAtomic(path, data, mode)
}

// JSONValue is the value at path in JSON with comments, nil when there is
// none.
func JSONValue(data []byte, path ...string) any {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	clean, _ := stripJSONC(data)
	var v any
	if json.Unmarshal(clean, &v) != nil {
		return nil
	}
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}
