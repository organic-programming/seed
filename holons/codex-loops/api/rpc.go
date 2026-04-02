package api

import (
	"context"

	codexloopsv1 "github.com/organic-programming/codex-loops/gen/go/v1"
)

type RPCHandler struct {
	codexloopsv1.UnimplementedCodexLoopsServiceServer
}

func (RPCHandler) Run(ctx context.Context, req *codexloopsv1.RunRequest) (*codexloopsv1.RunResponse, error) {
	return runContext(ctx, req)
}

func (RPCHandler) Enqueue(ctx context.Context, req *codexloopsv1.EnqueueRequest) (*codexloopsv1.EnqueueResponse, error) {
	return enqueueContext(ctx, req)
}

func (RPCHandler) List(ctx context.Context, req *codexloopsv1.ListRequest) (*codexloopsv1.ListResponse, error) {
	return listContext(ctx, req)
}

func (RPCHandler) Status(ctx context.Context, req *codexloopsv1.StatusRequest) (*codexloopsv1.StatusResponse, error) {
	return statusContext(ctx, req)
}

func (RPCHandler) Drop(ctx context.Context, req *codexloopsv1.DropRequest) (*codexloopsv1.DropResponse, error) {
	return dropContext(ctx, req)
}

func (RPCHandler) Resume(ctx context.Context, req *codexloopsv1.ResumeRequest) (*codexloopsv1.ResumeResponse, error) {
	return resumeContext(ctx, req)
}

func (RPCHandler) Skip(ctx context.Context, req *codexloopsv1.SkipRequest) (*codexloopsv1.SkipResponse, error) {
	return skipContext(ctx, req)
}

func (RPCHandler) Abort(ctx context.Context, req *codexloopsv1.AbortRequest) (*codexloopsv1.AbortResponse, error) {
	return abortContext(ctx, req)
}

func (RPCHandler) ReEnqueue(ctx context.Context, req *codexloopsv1.ReEnqueueRequest) (*codexloopsv1.ReEnqueueResponse, error) {
	return reEnqueueContext(ctx, req)
}

func (RPCHandler) Log(ctx context.Context, req *codexloopsv1.LogRequest) (*codexloopsv1.LogResponse, error) {
	return logContext(ctx, req)
}
