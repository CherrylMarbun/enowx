// Command enowxd is the enowX daemon for servers. On desktops the app runs
// the same daemon as `enowx daemon`.
//
//	enowxd serve    serve the per-user socket (~/.enowx/enowxd.sock)
//	enowxd bridge   pipe stdin/stdout to the socket, starting the daemon if needed
//	enowxd stop     stop the running daemon, ending its sessions
//	enowxd version  print the version
package main

import (
	"fmt"
	"os"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/transport"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = daemon.Run()
	case "bridge":
		err = daemon.Bridge("serve")
	case "stop":
		err = stop()
	case "version":
		fmt.Println(daemon.Version)
	default:
		fmt.Fprintf(os.Stderr, "usage: enowxd [serve|bridge|stop|version]\n")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "enowxd:", err)
		os.Exit(1)
	}
}

// stop asks the daemon on this machine for its PID and stops it. It works
// with daemons of other versions, which is how upgrades replace them.
func stop() error {
	path, err := transport.SocketPath()
	if err != nil {
		return err
	}
	conn, err := transport.DialLocal(path, nil)
	if err != nil {
		return nil // nothing is running
	}
	c, err := client.New(conn, nil)
	if err != nil {
		return err
	}
	info, err := c.Info()
	c.Close()
	if err != nil {
		return err
	}
	return transport.StopDaemon(info.PID, path)
}
