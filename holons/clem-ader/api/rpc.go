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

func (RPCHandler) TestBouquet(ctx context.Context, req *aderv1.BouquetRequest) (*aderv1.BouquetResponse, error) {
	return testBouquetContext(ctx, req)
}

func (RPCHandler) Archive(ctx context.Context, req *aderv1.ArchiveRequest) (*aderv1.ArchiveResponse, error) {
	return archiveContext(ctx, req)
}

func (RPCHandler) ArchiveBouquet(ctx context.Context, req *aderv1.ArchiveBouquetRequest) (*aderv1.ArchiveBouquetResponse, error) {
	return archiveBouquetContext(ctx, req)
}

func (RPCHandler) Cleanup(ctx context.Context, req *aderv1.CleanupRequest) (*aderv1.CleanupResponse, error) {
	return cleanupContext(ctx, req)
}

func (RPCHandler) History(ctx context.Context, req *aderv1.HistoryRequest) (*aderv1.HistoryResponse, error) {
	return historyContext(ctx, req)
}

func (RPCHandler) BouquetHistory(ctx context.Context, req *aderv1.BouquetHistoryRequest) (*aderv1.BouquetHistoryResponse, error) {
	return bouquetHistoryContext(ctx, req)
}

func (RPCHandler) ShowHistory(ctx context.Context, req *aderv1.ShowHistoryRequest) (*aderv1.ShowHistoryResponse, error) {
	return showHistoryContext(ctx, req)
}

func (RPCHandler) ShowBouquetHistory(ctx context.Context, req *aderv1.ShowBouquetHistoryRequest) (*aderv1.ShowBouquetHistoryResponse, error) {
	return showBouquetHistoryContext(ctx, req)
}

func (RPCHandler) Promote(ctx context.Context, req *aderv1.PromoteRequest) (*aderv1.PromoteResponse, error) {
	return promoteContext(ctx, req)
}

func (RPCHandler) Downgrade(ctx context.Context, req *aderv1.DowngradeRequest) (*aderv1.DowngradeResponse, error) {
	return downgradeContext(ctx, req)
}
