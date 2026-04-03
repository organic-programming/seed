package api

import (
	"context"

	jamesloopsv1 "github.com/organic-programming/james-loops/gen/go/v1"
)

type RPCHandler struct {
	jamesloopsv1.UnimplementedJamesLoopsServiceServer
}

func (RPCHandler) Run(ctx context.Context, req *jamesloopsv1.RunRequest) (*jamesloopsv1.RunResponse, error) {
	return runContext(ctx, req)
}

func (RPCHandler) Enqueue(ctx context.Context, req *jamesloopsv1.EnqueueRequest) (*jamesloopsv1.EnqueueResponse, error) {
	return enqueueContext(ctx, req)
}

func (RPCHandler) List(ctx context.Context, req *jamesloopsv1.ListRequest) (*jamesloopsv1.ListResponse, error) {
	return listContext(ctx, req)
}

func (RPCHandler) Status(ctx context.Context, req *jamesloopsv1.StatusRequest) (*jamesloopsv1.StatusResponse, error) {
	return statusContext(ctx, req)
}

func (RPCHandler) Drop(ctx context.Context, req *jamesloopsv1.DropRequest) (*jamesloopsv1.DropResponse, error) {
	return dropContext(ctx, req)
}

func (RPCHandler) Resume(ctx context.Context, req *jamesloopsv1.ResumeRequest) (*jamesloopsv1.ResumeResponse, error) {
	return resumeContext(ctx, req)
}

func (RPCHandler) Skip(ctx context.Context, req *jamesloopsv1.SkipRequest) (*jamesloopsv1.SkipResponse, error) {
	return skipContext(ctx, req)
}

func (RPCHandler) Abort(ctx context.Context, req *jamesloopsv1.AbortRequest) (*jamesloopsv1.AbortResponse, error) {
	return abortContext(ctx, req)
}

func (RPCHandler) ReEnqueue(ctx context.Context, req *jamesloopsv1.ReEnqueueRequest) (*jamesloopsv1.ReEnqueueResponse, error) {
	return reEnqueueContext(ctx, req)
}

func (RPCHandler) Log(ctx context.Context, req *jamesloopsv1.LogRequest) (*jamesloopsv1.LogResponse, error) {
	return logContext(ctx, req)
}
