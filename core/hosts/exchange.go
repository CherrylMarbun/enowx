package hosts

import (
	"encoding/json"
	"fmt"
	"slices"
)

// A host list goes from one device to another as a file the user keeps
// (in their dotfiles, say): the hosts as stored, which holds no secret
// (passwords stay in each device's keychain).

// exchangeVersion is the list file's layout.
const exchangeVersion = 1

type exchange struct {
	Version int    `json:"version"`
	Hosts   []Host `json:"hosts"`
}

// Export returns the list as a file's contents.
func (s *Store) Export() ([]byte, error) {
	list := s.List()
	for i := range list {
		list[i].ID = "" // each device gives its own
	}
	return json.MarshalIndent(exchange{Version: exchangeVersion, Hosts: list}, "", "  ")
}

// Import adds the hosts of a list file that aren't in the list yet (by
// address), and returns how many it added.
func (s *Store) Import(data []byte) (int, error) {
	var ex exchange
	if err := json.Unmarshal(data, &ex); err != nil {
		return 0, fmt.Errorf("hosts: not a host list: %w", err)
	}
	if ex.Version > exchangeVersion {
		return 0, fmt.Errorf("hosts: the list is from a newer enowX")
	}
	added := 0
	for _, h := range ex.Hosts {
		if h.Address == "" || slices.ContainsFunc(s.List(), func(o Host) bool { return o.Address == h.Address }) {
			continue
		}
		if _, err := s.Add(h); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}
