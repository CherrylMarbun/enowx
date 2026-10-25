package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// The app alone reaches other hosts: what an agent asks across hosts
// (copying a project to one, syncing it now) goes to the app attached
// here, the one serving Browsers, which does it and answers.

// appRequests are the requests sent to apps and waiting for answers.
type appRequests struct {
	mu      sync.Mutex
	pending map[string]chan proto.AppReply
}

// askApp has the latest app attached do kind with params, and returns
// its result, waiting at most timeout.
func (s *Server) askApp(kind string, params any, timeout time.Duration) (json.RawMessage, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	s.browsers.mu.Lock()
	var app *conn
	if n := len(s.browsers.serving); n > 0 {
		app = s.browsers.serving[n-1]
	}
	s.browsers.mu.Unlock()
	if app == nil {
		return nil, errors.New("no enowX app is connected to this host: only the app reaches other hosts, open it")
	}
	id := make([]byte, 8)
	_, _ = rand.Read(id)
	req := proto.AppRequest{ID: hex.EncodeToString(id), Kind: kind, Params: raw}
	ch := make(chan proto.AppReply, 1)
	r := &s.appReqs
	r.mu.Lock()
	if r.pending == nil {
		r.pending = map[string]chan proto.AppReply{}
	}
	r.pending[req.ID] = ch
	r.mu.Unlock()
	data, _ := json.Marshal(req)
	ev, _ := json.Marshal(proto.Event{Name: proto.EventAppRequest, Data: data})
	app.send(proto.Frame{Type: proto.TypeEvent, Payload: ev})
	select {
	case rep := <-ch:
		if rep.Error != "" {
			return nil, errors.New(rep.Error)
		}
		return rep.Result, nil
	case <-time.After(timeout):
		r.mu.Lock()
		delete(r.pending, req.ID)
		r.mu.Unlock()
		return nil, errors.New("the app didn't answer in time")
	}
}

func (r *appRequests) reply(rep proto.AppReply) {
	r.mu.Lock()
	ch := r.pending[rep.ID]
	delete(r.pending, rep.ID)
	r.mu.Unlock()
	if ch != nil {
		ch <- rep
	}
}

// daemonIDFile names this daemon's folder for good (DaemonInfo.ID).
const daemonIDFile = "daemon-id"

// loadDaemonID reads the daemon's ID in dir, made the first time.
func loadDaemonID(dir string) string {
	path := filepath.Join(dir, daemonIDFile)
	if data, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(data)); len(id) == 16 {
			return id
		}
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(path, []byte(id+"\n"), 0o600)
	return id
}
