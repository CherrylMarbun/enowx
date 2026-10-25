package cli

import (
	"errors"
	"fmt"
	"strings"
)

// Canvases shared by a read-only link, served by the host on this device
// and the user's tailnet.

var shareCommands = []string{"share", "unshare", "shares"}

const shareUsage = `  share CANVAS                     a read-only link to it, for this device and your tailnet's
  unshare CANVAS | shares          stop its link | the links`

func (r runner) canvasShare(cmd string, args []string) error {
	if cmd == "shares" {
		list, err := r.c.CanvasShares()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(r.out, "no canvas is shared")
		}
		for _, x := range list {
			fmt.Fprintf(r.out, "%s\t%s\t%s\n", x.Canvas, x.Title, strings.Join(x.URLs, " "))
		}
		return nil
	}
	if len(args) != 1 {
		return errors.New("usage: enowx-app canvas share CANVAS | unshare CANVAS | shares")
	}
	c, err := r.findCanvas(args[0])
	if err != nil {
		return err
	}
	if cmd == "unshare" {
		if _, err := r.c.ShareCanvas(c.ID, true); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "%q is no longer shared\n", c.Title)
		return nil
	}
	x, err := r.c.ShareCanvas(c.ID, false)
	if err != nil {
		return err
	}
	for _, u := range x.URLs {
		fmt.Fprintln(r.out, u)
	}
	fmt.Fprintf(r.out, "read-only, as it is when opened (it reloads as it changes); whoever has the link on this device or your tailnet sees it, until \"canvas unshare %s\"\n", c.ID)
	return nil
}
