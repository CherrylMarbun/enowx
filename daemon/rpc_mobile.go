package daemon

import (
	"github.com/enowx/enowx/core/android"
	"github.com/enowx/enowx/proto"
)

var androidHandlers = map[string]handler{
	proto.MethodAndroidStatus: bare(func(c *conn) (any, error) { return c.srv.androidStatus(), nil }),
	proto.MethodAndroidSetup:  with(func(c *conn, p proto.AndroidSetupParams) (any, error) { return c.srv.androidSetup(p) }),
	proto.MethodAndroidStart:  with(func(c *conn, p proto.AndroidDeviceParams) (any, error) { return ok, c.srv.androidStart(p) }),
	proto.MethodAndroidStop:   with(func(c *conn, p proto.AndroidDeviceParams) (any, error) { return ok, c.srv.androidStop(p) }),
	proto.MethodAndroidConnect: with(func(c *conn, p proto.AndroidConnectParams) (any, error) {
		return ok, c.srv.androidConnect(p)
	}),
	proto.MethodAndroidLogcat: with(func(c *conn, p proto.AndroidLogcatParams) (any, error) { return c.srv.androidLogcat(p) }),
	proto.MethodAndroidRun:    with(func(c *conn, p proto.AndroidRunParams) (any, error) { return c.srv.androidRun(p) }),
	proto.MethodAndroidProject: with(func(c *conn, p proto.AndroidProjectParams) (any, error) {
		return android.Project(p.Dir), nil
	}),
	proto.MethodAndroidADBOpen:   bare(func(c *conn) (any, error) { return c.adbOpen() }),
	proto.MethodAndroidADBListen: bare(func(c *conn) (any, error) { return c.adbListen() }),
	proto.MethodAndroidADBAccept: with(func(c *conn, p proto.AndroidADBConn) (any, error) { return c.adbAccept(p) }),
	proto.MethodAndroidADBClose: with(func(c *conn, p proto.AndroidADBListener) (any, error) {
		c.adbClose(p.ID)
		return ok, nil
	}),
	proto.MethodAndroidForwardOpen: with(func(c *conn, p proto.AndroidForwardParams) (any, error) { return c.forwardOpen(p) }),
}
