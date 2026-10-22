// Command enowx is the enowX desktop app.
//
//	enowx                            connect to the local daemon (starting it)
//	enowx --remote user@host[:port]  connect to a server's daemon over SSH
//	enowx --project DIR              also open DIR as a project
//	enowx daemon                     run the local daemon (started by the app)
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/transport"
	enowxui "github.com/enowx/enowx/ui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		if err := daemon.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "enowx daemon:", err)
			os.Exit(1)
		}
		return
	}

	remote := flag.String("remote", "", "connect to the daemon on `user@host[:port]` over SSH")
	project := flag.String("project", "", "open `DIR` (a path on the daemon's host) as a project")
	flag.Parse()

	rw, host, err := connect(*remote)
	if err != nil {
		log.Fatal(err)
	}
	app := enowxui.New(host)
	c, err := client.New(rw, app.HandleEvent)
	if err != nil {
		log.Fatal(err)
	}
	app.SetClient(c)
	if *project != "" {
		if _, err := c.AddProject(*project); err != nil {
			log.Fatal(err)
		}
	}

	mygo.App.WhenReady(func() { app.Open() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func connect(remote string) (io.ReadWriteCloser, enowxui.Host, error) {
	if remote != "" {
		target, err := transport.ParseSSHTarget(remote)
		if err != nil {
			return nil, enowxui.Host{}, err
		}
		rw, err := transport.DialSSH(target, "")
		return rw, enowxui.Host{Name: target.User + "@" + target.Host, Detail: "SSH", Remote: true}, err
	}
	path, err := transport.SocketPath()
	if err != nil {
		return nil, enowxui.Host{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, enowxui.Host{}, err
	}
	conn, err := transport.DialLocal(path, exec.Command(exe, "daemon"))
	name := "This PC"
	if runtime.GOOS == "darwin" {
		name = "This Mac"
	}
	return conn, enowxui.Host{Name: name, Detail: "Local"}, err
}
