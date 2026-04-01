package api

import (
	"context"

	aderv1 "github.com/organic-programming/clem-ader/gen/go/v1"
)

type RPCHandler struct {
	aderv1.UnimplementedAderServiceServer
}

func (RPCHandler) Test(ctx context.Context, req *aderv1.TestRequest) (*aderv1.TestResponse, error) {
	return testContext(ctx, req)
}

func (RPCHandler) Archive(ctx context.Context, req *aderv1.ArchiveRequest) (*aderv1.ArchiveResponse, error) {
	return archiveContext(ctx, req)
}

func (RPCHandler) Cleanup(ctx context.Context, req *aderv1.CleanupRequest) (*aderv1.CleanupResponse, error) {
	return cleanupContext(ctx, req)
}

func (RPCHandler) ListRuns(ctx context.Context, req *aderv1.ListRunsRequest) (*aderv1.ListRunsResponse, error) {
	return listRunsContext(ctx, req)
}

func (RPCHandler) ShowRun(ctx context.Context, req *aderv1.ShowRunRequest) (*aderv1.ShowRunResponse, error) {
	return showRunContext(ctx, req)
}
