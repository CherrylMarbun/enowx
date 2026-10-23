package daemon

import "github.com/enowx/enowx/proto"

// creditHandlers answer from the last reading at once: requests on a
// connection run in order, and a reading takes seconds.
var creditHandlers = map[string]handler{
	proto.MethodCreditsGet: with(func(c *conn, p proto.CreditsParams) (any, error) {
		return c.srv.credits.Report(p.Refresh), nil
	}),
}
