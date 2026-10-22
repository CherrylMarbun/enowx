//go:build !darwin

package tailnet

import (
	"context"
	"errors"
)

// localStatus is only needed on macOS; elsewhere the CLI works.
func localStatus(context.Context) ([]byte, error) {
	return nil, errors.New("tailnet: no local API client on this platform")
}
