package intercept

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// pauseLimit is how long an exchange waits at a breakpoint before it
// goes on unchanged, so a forgotten one doesn't hold an app forever.
const pauseLimit = 5 * time.Minute

// pause is an exchange waiting at a breakpoint.
type pause struct {
	info proto.TrafficPaused
	ch   chan resume
	once sync.Once
}

// resume is how a paused exchange goes on.
type resume struct {
	drop bool
	edit *proto.TrafficEdit
}

func (ps *pause) resolve(r resume) { ps.once.Do(func() { ps.ch <- r }) }

// hold pauses an exchange until the user resumes or drops it, the client
// goes away, or pauseLimit passes.
func (p *Proxy) hold(ctx context.Context, info proto.TrafficPaused) resume {
	info.Since = time.Now()
	ps := &pause{info: info, ch: make(chan resume, 1)}
	p.mu.Lock()
	if p.pauses == nil {
		p.pauses = map[int64]*pause{}
	}
	p.pauses[info.ID] = ps
	p.mu.Unlock()
	p.pausedChanged()
	var r resume
	select {
	case r = <-ps.ch:
	case <-ctx.Done():
		r = resume{drop: true}
	case <-time.After(pauseLimit):
	}
	p.mu.Lock()
	delete(p.pauses, info.ID)
	p.mu.Unlock()
	p.pausedChanged()
	return r
}

// Paused lists the exchanges waiting at breakpoints, oldest first.
func (p *Proxy) Paused() []proto.TrafficPaused {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]proto.TrafficPaused, 0, len(p.pauses))
	for _, ps := range p.pauses {
		out = append(out, ps.info)
	}
	slices.SortFunc(out, func(a, b proto.TrafficPaused) int { return a.Since.Compare(b.Since) })
	return out
}

// Resume lets paused exchange id go on with edit's changes, or drops it.
func (p *Proxy) Resume(id int64, drop bool, edit *proto.TrafficEdit) error {
	p.mu.Lock()
	ps, ok := p.pauses[id]
	p.mu.Unlock()
	if !ok {
		return errors.New("intercept: that exchange isn't paused")
	}
	ps.resolve(resume{drop: drop, edit: edit})
	return nil
}

func (p *Proxy) pausedChanged() {
	if p.OnPaused != nil {
		p.OnPaused(p.Paused())
	}
}

// holdExchange holds ex at a breakpoint, its state "paused" meanwhile.
func (p *Proxy) holdExchange(ctx context.Context, ex *Exchange, info proto.TrafficPaused) resume {
	ex.update(func(ex *Exchange) { ex.sum.State = proto.TrafficHeld })
	r := p.hold(ctx, info)
	ex.update(func(ex *Exchange) { ex.sum.State = proto.TrafficPending })
	return r
}
