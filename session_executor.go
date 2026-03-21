package main

import (
	"context"
	"log/slog"
	"sync"

	"github.com/lightpanda-io/gomcp/mcp"
)

const defaultStatelessParallelism = 2

type sessionExecutor struct {
	srv          *MCPServer
	statefulConn *MCPConn
	sendMu       sync.Mutex
	statefulMu   sync.Mutex
	statelessSem chan struct{}
	wg           sync.WaitGroup
}

func newSessionExecutor(srv *MCPServer) *sessionExecutor {
	return &sessionExecutor{
		srv:          srv,
		statefulConn: srv.NewConn(),
		statelessSem: make(chan struct{}, defaultStatelessParallelism),
	}
}

func (e *sessionExecutor) Close() {
	e.wg.Wait()
	if e.statefulConn != nil {
		e.statefulConn.Close()
	}
}

func (e *sessionExecutor) lockedSend(send SendFn) SendFn {
	return func(event string, data any) error {
		e.sendMu.Lock()
		defer e.sendMu.Unlock()
		return send(event, data)
	}
}

func (e *sessionExecutor) Handle(ctx context.Context, rreq mcp.Request, send SendFn) error {
	lockedSend := e.lockedSend(send)

	toolReq, ok := rreq.(mcp.ToolsCallRequest)
	if !ok {
		return e.srv.Handle(ctx, rreq, e.statefulConn, lockedSend)
	}

	e.wg.Add(1)
	switch e.srv.toolMode(toolReq.Params.Name) {
	case toolExecutionStateless:
		go e.runStatelessTool(ctx, toolReq, lockedSend)
	default:
		go e.runStatefulTool(ctx, toolReq, lockedSend)
	}

	return nil
}

func (e *sessionExecutor) runStatefulTool(ctx context.Context, req mcp.ToolsCallRequest, send SendFn) {
	defer e.wg.Done()

	e.statefulMu.Lock()
	defer e.statefulMu.Unlock()

	res, err := e.srv.CallTool(ctx, e.statefulConn, req)
	if err := e.srv.sendToolResult(send, req.Id, res, err); err != nil {
		slog.Error("send tool result", slog.String("name", req.Params.Name), slog.Any("err", err))
	}
}

func (e *sessionExecutor) runStatelessTool(ctx context.Context, req mcp.ToolsCallRequest, send SendFn) {
	defer e.wg.Done()

	e.statelessSem <- struct{}{}
	defer func() {
		<-e.statelessSem
	}()

	conn := e.srv.NewConn()
	defer conn.Close()

	res, err := e.srv.CallTool(ctx, conn, req)
	if err := e.srv.sendToolResult(send, req.Id, res, err); err != nil {
		slog.Error("send tool result", slog.String("name", req.Params.Name), slog.Any("err", err))
	}
}
