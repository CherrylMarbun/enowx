// Command enowx is the enowX desktop app.
//
//	enowx                          connect to the local daemon (starting it)
//	enowx --remote user@host[:port] connect to a server's daemon over SSH
//	enowx --repo DIR --agent claude --count 4
//	                               start an agent in each of 4 new worktrees
//	enowx daemon                   run the local daemon (started by the app)
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
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
	repo := flag.String("repo", "", "git repository to create agent worktrees in")
	agent := flag.String("agent", "", "agent command to run in each worktree, e.g. claude")
	count := flag.Int("count", 1, "number of agent worktrees with --repo")
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
	if *repo != "" {
		if err := startAgents(c, *repo, *agent, *count); err != nil {
			log.Fatal(err)
		}
	}

	mygo.App.WhenReady(func() { app.Open() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func connect(remote string) (io.ReadWriteCloser, string, error) {
	if remote != "" {
		target, err := transport.ParseSSHTarget(remote)
		if err != nil {
			return nil, "", err
		}
		rw, err := transport.DialSSH(target, "")
		return rw, target.User + "@" + target.Host, err
	}
	path, err := transport.SocketPath()
	if err != nil {
		return nil, "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, "", err
	}
	conn, err := transport.DialLocal(path, exec.Command(exe, "daemon"))
	host, _ := os.Hostname()
	return conn, "This machine (" + host + ")", err
}

// startAgents creates count worktrees of repo and starts agent in each one
// that doesn't have a running session yet, so relaunching reuses them.
func startAgents(c *client.Client, repo, agent string, count int) error {
	existing, err := c.Sessions()
	if err != nil {
		return err
	}
	running := map[string]bool{}
	for _, s := range existing {
		if !s.Exited {
			running[s.Dir] = true
		}
	}
	var command []string
	if agent != "" {
		command = []string{agent}
	}
	for i := 1; i <= count; i++ {
		wt, err := c.CreateWorktree(proto.WorktreeCreateParams{Repo: repo, Name: fmt.Sprintf("agent-%d", i)})
		if err != nil {
			return err
		}
		if running[wt.Path] {
			continue
		}
		if _, err := c.CreateSession(proto.SessionCreateParams{Command: command, Dir: wt.Path}); err != nil {
			return err
		}
	}
	return nil
}
