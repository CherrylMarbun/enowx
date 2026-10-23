package transport

import (
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
)

// Forward listens on this machine's loopback, on port when it is free and
// on a port of the system's choosing otherwise, and carries each
// connection to one dial makes: a server's dev server shows on localhost.
// Closing the listener stops taking connections; those open go on until
// either side ends them.
func Forward(port int, dial func() (net.Conn, error)) (net.Listener, error) {
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		if l, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return nil, err
		}
	}
	go func() {
		for {
			conn, err := l.Accept()
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if err != nil {
				continue
			}
			go carry(conn, dial)
		}
	}()
	return l, nil
}

// carry copies between local and the connection dial makes, both ways,
// until both are done.
func carry(local net.Conn, dial func() (net.Conn, error)) {
	defer local.Close()
	remote, err := dial()
	if err != nil {
		return
	}
	defer remote.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	pipe := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		// Pass the end of one direction on, so the other side can finish.
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		} else {
			_ = dst.Close()
		}
	}
	go pipe(remote, local)
	go pipe(local, remote)
	wg.Wait()
}
