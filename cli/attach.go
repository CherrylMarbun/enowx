package cli

import (
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/term"
)

// detachKey (Ctrl+]) leaves an attached session, which keeps running.
const detachKey = 0x1d

// attach uses a session's terminal in this one: what it shows comes here,
// what is typed here goes to it, until Ctrl+] or its end.
func (r runner) attach(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: enowx attach SESSION")
	}
	s, err := r.session(args[0])
	if err != nil {
		return err
	}
	in := int(os.Stdin.Fd())
	if !term.IsTerminal(in) {
		return errors.New("attach: standard input isn't a terminal")
	}
	st, err := r.c.Attach(s.ID)
	if err != nil {
		return err
	}
	defer st.Close()
	if cols, rows, err := term.GetSize(in); err == nil {
		_ = st.Resize(cols, rows)
	}
	old, err := term.MakeRaw(in)
	if err != nil {
		return err
	}
	defer term.Restore(in, old)

	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(r.out, st)
		done <- err
	}()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			for i := 0; i < n; i++ {
				if buf[i] == detachKey {
					if i > 0 {
						_, _ = st.Write(buf[:i])
					}
					done <- nil
					return
				}
			}
			if n > 0 {
				if _, werr := st.Write(buf[:n]); werr != nil {
					done <- werr
					return
				}
			}
			if err != nil {
				done <- err
				return
			}
		}
	}()
	// The terminal here may change size meanwhile: follow it.
	go func() {
		cols, rows, _ := term.GetSize(in)
		for range time.Tick(500 * time.Millisecond) {
			if c, r, err := term.GetSize(in); err == nil && (c != cols || r != rows) {
				cols, rows = c, r
				_ = st.Resize(cols, rows)
			}
		}
	}()
	err = <-done
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return err
}
