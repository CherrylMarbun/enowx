package daemon

import (
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

var daemonHandlers = map[string]handler{
	proto.MethodDaemonInfo:  bare(func(c *conn) (any, error) { return c.srv.info(), nil }),
	proto.MethodDaemonStats: bare(func(c *conn) (any, error) { return c.srv.stats(), nil }),
}

var agentHandlers = map[string]handler{
	proto.MethodAgentList: bare(func(c *conn) (any, error) {
		// Agents installed since the daemon started show up.
		refreshLoginPath(c.srv.logger())
		return agent.List(), nil
	}),
}
