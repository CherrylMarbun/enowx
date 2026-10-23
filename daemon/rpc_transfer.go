package daemon

import "github.com/enowx/enowx/proto"

var transferHandlers = map[string]handler{
	proto.MethodTransferPack: with(func(c *conn, p proto.TransferPackParams) (any, error) {
		return c.srv.pack(p)
	}),
	proto.MethodTransferRead: with(func(c *conn, p proto.TransferReadParams) (any, error) {
		return c.srv.readTransfer(p)
	}),
	proto.MethodTransferWrite: with(func(c *conn, p proto.TransferPart) (any, error) {
		return ok, c.srv.writeTransfer(p)
	}),
	proto.MethodTransferUnpack: with(func(c *conn, p proto.TransferUnpackParams) (any, error) {
		return c.srv.unpack(p)
	}),
	proto.MethodTransferDiscard: with(func(c *conn, p proto.TransferRef) (any, error) {
		return ok, c.srv.discardTransfer(p.ID)
	}),
}
