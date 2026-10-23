// Package credits reads what each agent account signed in on this machine
// has left of its plan, with the agent's own login: Claude Code's and
// Codex's tokens go to the vendor's usage endpoint, the only place they are
// sent, and Kiro CLI is asked for its /usage. Tokens are read when needed,
// never kept, logged or refreshed: refreshing one would sign the agent out.
package credits

import (
	"context"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// Readings older than staleAfter are read again when asked for; a refresh
// asked for sooner than minGap after the last one waits for it.
const (
	staleAfter  = 5 * time.Minute
	minGap      = 30 * time.Second
	readTimeout = 45 * time.Second
)

// provider reads the accounts of one agent. It returns none when the
// agent is not installed or not signed in.
type provider func(ctx context.Context) []proto.CreditAccount

var providers = []provider{readClaude, readCodex, readKiro}

// Checker keeps the latest reading. Its fields are guarded by mu; the
// reading itself runs on a goroutine of its own, one at a time.
type Checker struct {
	mu       sync.Mutex
	report   proto.CreditsReport
	started  time.Time
	checking bool
}

// Report returns the latest reading, and starts a new one when it is
// stale, or when refresh asks and the last one is not too recent.
func (c *Checker) Report(refresh bool) proto.CreditsReport {
	c.mu.Lock()
	defer c.mu.Unlock()
	since := time.Since(c.started)
	if !c.checking && (since > staleAfter || refresh && since > minGap) {
		c.checking, c.started = true, time.Now()
		go c.read()
	}
	r := c.report
	r.Checking = c.checking
	return r
}

func (c *Checker) read() {
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	results := make([][]proto.CreditAccount, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = p(ctx)
		}()
	}
	wg.Wait()
	accounts := []proto.CreditAccount{}
	for _, r := range results {
		accounts = append(accounts, r...)
	}
	c.mu.Lock()
	c.report = proto.CreditsReport{Accounts: accounts, Checked: time.Now()}
	c.checking = false
	c.mu.Unlock()
}
