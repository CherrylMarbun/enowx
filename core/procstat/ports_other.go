//go:build !linux && !darwin

package procstat

import "errors"

// Listening isn't read on this system.
func Listening(pids []int) ([]Listener, error) { return nil, errors.ErrUnsupported }
