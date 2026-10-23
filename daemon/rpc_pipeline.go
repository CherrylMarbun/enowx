package daemon

import "github.com/enowx/enowx/proto"

// pipelineHandlers start, list and stop pipelines (daemon/pipeline.go).
var pipelineHandlers = map[string]handler{
	proto.MethodPipelineStart: with(func(c *conn, p proto.PipelineStartParams) (any, error) {
		return c.srv.startPipeline(p)
	}),
	proto.MethodPipelineList: bare(func(c *conn) (any, error) {
		return c.srv.listPipelines(), nil
	}),
	proto.MethodPipelineCancel: with(func(c *conn, p proto.PipelineRef) (any, error) {
		return ok, c.srv.cancelPipeline(p.ID)
	}),
	proto.MethodPipelineRemove: with(func(c *conn, p proto.PipelineRef) (any, error) {
		return ok, c.srv.removePipeline(p.ID)
	}),
}
