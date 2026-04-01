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

func (RPCHandler) History(ctx context.Context, req *aderv1.HistoryRequest) (*aderv1.HistoryResponse, error) {
	return historyContext(ctx, req)
}

func (RPCHandler) ShowHistory(ctx context.Context, req *aderv1.ShowHistoryRequest) (*aderv1.ShowHistoryResponse, error) {
	return showHistoryContext(ctx, req)
}

func (RPCHandler) Downgrade(ctx context.Context, req *aderv1.DowngradeRequest) (*aderv1.DowngradeResponse, error) {
	return downgradeContext(ctx, req)
}
