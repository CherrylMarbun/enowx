package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/transport"
	"github.com/enowx/enowx/ui/textfmt"
)

// Quitting leaves this device's daemon running, so agents and automations
// go on without the app. "Quit and stop all agents", or the setting to
// always do so, stops its agents first; their sessions stay, to continue.

// watchQuit stops this device's agents as the app quits, when asked.
func (a *App) watchQuit() {
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		if a.stopOnQuit || a.state.StopAgentsOnQuit {
			a.stopLocalAgents()
		}
	})
}

// quitAndStopAgents quits the app, stopping this device's agents.
func (a *App) quitAndStopAgents() {
	a.stopOnQuit = true
	mygo.App.Quit()
}

// stopLocalAgents asks this device's daemon, if it runs, to stop its
// agents, waiting a moment at most: quitting shouldn't hang on it.
func (a *App) stopLocalAgents() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		cl, err := localClient()
		if err != nil {
			return
		}
		defer cl.Close()
		_, _ = cl.StopAgents()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// localClient connects to this device's daemon without starting one.
func localClient() (*client.Client, error) {
	path, err := transport.SocketPath()
	if err != nil {
		return nil, err
	}
	conn, err := transport.DialLocal(path, nil)
	if err != nil {
		return nil, err
	}
	return client.New(conn, nil)
}

// stopAgentsHere stops the agents of the connected host now.
func (a *App) stopAgentsHere() {
	cl, name := a.client, a.host.Name
	if cl == nil {
		return
	}
	go func() {
		n, err := cl.StopAgents()
		a.onMain(func() {
			if err != nil {
				a.status = "Couldn't stop the agents: " + client.Explain(err)
				return
			}
			a.notify(stoppedAgents(n) + " on " + name)
		})
	}()
}

func stoppedAgents(n int) string {
	if n == 0 {
		return "No agent was running"
	}
	return fmt.Sprintf("Stopped %d %s", n, textfmt.Plural(n, "agent", "agents"))
}
