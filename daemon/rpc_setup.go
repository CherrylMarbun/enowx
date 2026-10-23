package daemon

import (
	"github.com/enowx/enowx/core/setup"
	"github.com/enowx/enowx/proto"
)

var setupHandlers = map[string]handler{
	proto.MethodSetupScan: bare(func(c *conn) (any, error) { return setup.Scan(), nil }),
}
