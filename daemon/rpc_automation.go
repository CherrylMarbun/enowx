package daemon

import (
	"time"

	"github.com/enowx/enowx/proto"
)

// automationHandlers keep the host's automations (daemon/automation.go).
var automationHandlers = map[string]handler{
	proto.MethodAutomationList: bare(func(c *conn) (any, error) { return c.srv.listAutomations(), nil }),
	proto.MethodAutomationSave: with(func(c *conn, p proto.Automation) (any, error) {
		return c.srv.saveAutomation(p)
	}),
	proto.MethodAutomationRemove: with(func(c *conn, p proto.AutomationRef) (any, error) {
		return ok, c.srv.removeAutomation(p.ID)
	}),
	proto.MethodAutomationRun: with(func(c *conn, p proto.AutomationRef) (any, error) {
		sess, err := c.srv.runAutomation(p.ID, time.Now())
		if err != nil {
			return nil, err
		}
		return sess.Info(), nil
	}),
}
