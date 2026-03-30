package api

import (
	"context"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

type RPCHandler struct {
	opv1.UnimplementedOPServiceServer
}

func (RPCHandler) Discover(_ context.Context, req *opv1.DiscoverRequest) (*opv1.DiscoverResponse, error) {
	return Discover(req)
}

func (RPCHandler) Invoke(ctx context.Context, req *opv1.InvokeRequest) (*opv1.InvokeResponse, error) {
	return invokeContext(ctx, req)
}

func (RPCHandler) CreateIdentity(_ context.Context, req *opv1.CreateIdentityRequest) (*opv1.CreateIdentityResponse, error) {
	return CreateIdentity(req)
}

func (RPCHandler) ListIdentities(_ context.Context, req *opv1.ListIdentitiesRequest) (*opv1.ListIdentitiesResponse, error) {
	return ListIdentities(req)
}

func (RPCHandler) ShowIdentity(_ context.Context, req *opv1.ShowIdentityRequest) (*opv1.ShowIdentityResponse, error) {
	return ShowIdentity(req)
}

func (RPCHandler) ListTemplates(_ context.Context, req *opv1.ListTemplatesRequest) (*opv1.ListTemplatesResponse, error) {
	return ListTemplates(req)
}

func (RPCHandler) GenerateTemplate(_ context.Context, req *opv1.GenerateTemplateRequest) (*opv1.GenerateTemplateResponse, error) {
	return GenerateTemplate(req)
}

func (RPCHandler) Check(_ context.Context, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return Check(req)
}

func (RPCHandler) Build(_ context.Context, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return Build(req)
}

func (RPCHandler) Test(_ context.Context, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return Test(req)
}

func (RPCHandler) Clean(_ context.Context, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return Clean(req)
}

func (RPCHandler) Install(_ context.Context, req *opv1.InstallRequest) (*opv1.InstallResponse, error) {
	return Install(req)
}

func (RPCHandler) Uninstall(_ context.Context, req *opv1.UninstallRequest) (*opv1.InstallResponse, error) {
	return Uninstall(req)
}

func (RPCHandler) Run(_ context.Context, req *opv1.RunRequest) (*opv1.RunResponse, error) {
	return Run(req)
}

func (RPCHandler) Inspect(_ context.Context, req *opv1.InspectRequest) (*opv1.InspectResponse, error) {
	return Inspect(req)
}

func (RPCHandler) RunSequence(_ context.Context, req *opv1.RunSequenceRequest) (*opv1.RunSequenceResponse, error) {
	return RunSequence(req)
}

func (RPCHandler) ModInit(_ context.Context, req *opv1.ModInitRequest) (*opv1.ModInitResponse, error) {
	return ModInit(req)
}

func (RPCHandler) ModAdd(_ context.Context, req *opv1.ModAddRequest) (*opv1.ModAddResponse, error) {
	return ModAdd(req)
}

func (RPCHandler) ModRemove(_ context.Context, req *opv1.ModRemoveRequest) (*opv1.ModRemoveResponse, error) {
	return ModRemove(req)
}

func (RPCHandler) ModTidy(_ context.Context, req *opv1.ModTidyRequest) (*opv1.ModTidyResponse, error) {
	return ModTidy(req)
}

func (RPCHandler) ModPull(_ context.Context, req *opv1.ModPullRequest) (*opv1.ModPullResponse, error) {
	return ModPull(req)
}

func (RPCHandler) ModUpdate(_ context.Context, req *opv1.ModUpdateRequest) (*opv1.ModUpdateResponse, error) {
	return ModUpdate(req)
}

func (RPCHandler) ModList(_ context.Context, req *opv1.ModListRequest) (*opv1.ModListResponse, error) {
	return ModList(req)
}

func (RPCHandler) ModGraph(_ context.Context, req *opv1.ModGraphRequest) (*opv1.ModGraphResponse, error) {
	return ModGraph(req)
}

func (RPCHandler) Tools(_ context.Context, req *opv1.ToolsRequest) (*opv1.ToolsResponse, error) {
	return Tools(req)
}

func (RPCHandler) Env(_ context.Context, req *opv1.EnvRequest) (*opv1.EnvResponse, error) {
	return Env(req)
}
