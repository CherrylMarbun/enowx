package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// Unslop in a session's Browser: one click reads the page shown (the
// project's, on localhost) and has the session's agent redesign the code
// where it looks AI-made, with nothing more to ask.

func (a *App) browserUnslop(s proto.SessionInfo) {
	cl := a.client
	if cl == nil || a.browserUnslopping[s.ID] {
		return
	}
	if a.browserUnslopping == nil {
		a.browserUnslopping = map[string]bool{}
	}
	a.browserUnslopping[s.ID] = true
	delete(a.browserUnslopErr, s.ID)
	go func() {
		u, err := cl.BrowserUnslop(proto.BrowserUnslopParams{Session: s.ID, Start: true})
		a.onMain(func() {
			delete(a.browserUnslopping, s.ID)
			if err != nil {
				if a.browserUnslopErr == nil {
					a.browserUnslopErr = map[string]string{}
				}
				a.browserUnslopErr[s.ID] = strings.TrimPrefix(client.Explain(err), "browser: ")
				return
			}
			a.sendWithFiles(s.ID, u.Ask, nil)
			a.notify(fmt.Sprintf("Unslop: %d/100, %d findings; the agent fixes the code", u.Score, len(u.Findings)))
		})
	}()
}

// browserUnslopEvent keeps a Browser's unslop as the daemon tells it.
func (a *App) browserUnslopEvent(data json.RawMessage) func() {
	var run proto.CanvasUnslopRun
	if json.Unmarshal(data, &run) != nil {
		return nil
	}
	return func() {
		if a.browserUnslops == nil {
			a.browserUnslops = map[string]proto.CanvasUnslopRun{}
		}
		a.browserUnslops[run.ID] = run
	}
}

// browserUnslopRun is session id's Browser's unslop, nil when none.
func (a *App) browserUnslopRun(id string) *proto.CanvasUnslopRun {
	// A run with no page is one that didn't start.
	if run, ok := a.browserUnslops[id]; ok && run.Frame != "" {
		return &run
	}
	return nil
}
