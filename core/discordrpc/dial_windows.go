//go:build windows

package discordrpc

import (
	"io"
	"os"
	"strconv"
)

// dial opens Discord's named pipe, discord-ipc-0 to 9.
func dial() (io.ReadWriteCloser, error) {
	for i := range 10 {
		if f, err := os.OpenFile(`\\.\pipe\discord-ipc-`+strconv.Itoa(i), os.O_RDWR, 0); err == nil {
			return f, nil
		}
	}
	return nil, ErrNotRunning
}
