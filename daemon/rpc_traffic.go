package daemon

import "github.com/enowx/enowx/proto"

var trafficHandlers = map[string]handler{
	proto.MethodTrafficStatus: bare(func(c *conn) (any, error) { return c.srv.trafficStatus(), nil }),
	proto.MethodTrafficCA:     bare(func(c *conn) (any, error) { return c.srv.trafficCA() }),
	proto.MethodTrafficAttach: with(func(c *conn, p proto.TrafficAttachParams) (any, error) { return c.srv.attach(p) }),
	proto.MethodTrafficDetach: with(func(c *conn, p proto.TrafficTargetRef) (any, error) { return ok, c.srv.detach(p.Source) }),
	proto.MethodTrafficAttached: bare(func(c *conn) (any, error) {
		if _, err := c.srv.trafficOn(); err != nil {
			return nil, err
		}
		return c.srv.trafficTargets(), nil
	}),
	proto.MethodTrafficEnv:         with(func(c *conn, p proto.TrafficEnvParams) (any, error) { return c.srv.trafficEnv(p) }),
	proto.MethodTrafficList:        with(func(c *conn, p proto.TrafficListParams) (any, error) { return c.srv.trafficList(p) }),
	proto.MethodTrafficGet:         with(func(c *conn, p proto.TrafficRef) (any, error) { return c.srv.trafficGet(p) }),
	proto.MethodTrafficBody:        with(func(c *conn, p proto.TrafficBodyParams) (any, error) { return c.srv.trafficBody(p) }),
	proto.MethodTrafficMessages:    with(func(c *conn, p proto.TrafficMessagesParams) (any, error) { return c.srv.trafficMessages(p) }),
	proto.MethodTrafficPin:         with(func(c *conn, p proto.TrafficPinParams) (any, error) { return ok, c.srv.trafficPin(p) }),
	proto.MethodTrafficClear:       with(func(c *conn, p proto.TrafficClearParams) (any, error) { return ok, c.srv.trafficClear(p) }),
	proto.MethodTrafficRules:       with(func(c *conn, p proto.TrafficRulesParams) (any, error) { return c.srv.trafficRules(p) }),
	proto.MethodTrafficRulesSet:    with(func(c *conn, p proto.TrafficRules) (any, error) { return c.srv.setTrafficRules(p) }),
	proto.MethodTrafficPaused:      bare(func(c *conn) (any, error) { return c.srv.trafficPaused() }),
	proto.MethodTrafficResume:      with(func(c *conn, p proto.TrafficResumeParams) (any, error) { return ok, c.srv.trafficResume(p) }),
	proto.MethodTrafficSend:        with(func(c *conn, p proto.TrafficSendParams) (any, error) { return c.srv.trafficSend(p) }),
	proto.MethodTrafficExport:      with(func(c *conn, p proto.TrafficExportParams) (any, error) { return c.srv.trafficExport(p) }),
	proto.MethodTrafficImport:      with(func(c *conn, p proto.TrafficImportParams) (any, error) { return c.srv.trafficImport(p) }),
	proto.MethodTrafficSettings:    bare(func(c *conn) (any, error) { return c.srv.trafficSettings() }),
	proto.MethodTrafficSetSettings: with(func(c *conn, p proto.TrafficSettings) (any, error) { return c.srv.setTrafficSettings(p) }),
	proto.MethodTrafficReverse:     with(func(c *conn, p proto.TrafficReverseParams) (any, error) { return c.srv.reverse(p) }),
	proto.MethodTrafficTunnel:      with(func(c *conn, p proto.TrafficTargetRef) (any, error) { return c.trafficTunnel(p) }),
}
