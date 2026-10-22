// Command enowxd is the enowX daemon for servers. On desktops the app runs
// the same daemon as `enowx daemon`.
//
//	enowxd serve    serve the per-user socket (~/.enowx/enowxd.sock)
//	enowxd bridge   pipe stdin/stdout to the socket, starting the daemon if needed
//	enowxd version  print the version
package main

import (
	"fmt"
	"os"

	"github.com/enowx/enowx/daemon"
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
	case "version":
		fmt.Println(daemon.Version)
	default:
		fmt.Fprintf(os.Stderr, "usage: enowxd [serve|bridge|version]\n")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "enowxd:", err)
		os.Exit(1)
	}
}
