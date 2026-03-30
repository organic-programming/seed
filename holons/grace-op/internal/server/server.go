// Package server implements OP's gRPC transport adapter.
package server

import (
	"context"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"

	"google.golang.org/grpc"
)

type Handler interface {
	Discover(context.Context, *opv1.DiscoverRequest) (*opv1.DiscoverResponse, error)
	Invoke(context.Context, *opv1.InvokeRequest) (*opv1.InvokeResponse, error)
	CreateIdentity(context.Context, *opv1.CreateIdentityRequest) (*opv1.CreateIdentityResponse, error)
	ListIdentities(context.Context, *opv1.ListIdentitiesRequest) (*opv1.ListIdentitiesResponse, error)
	ShowIdentity(context.Context, *opv1.ShowIdentityRequest) (*opv1.ShowIdentityResponse, error)
	ListTemplates(context.Context, *opv1.ListTemplatesRequest) (*opv1.ListTemplatesResponse, error)
	GenerateTemplate(context.Context, *opv1.GenerateTemplateRequest) (*opv1.GenerateTemplateResponse, error)
	Check(context.Context, *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error)
	Build(context.Context, *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error)
	Test(context.Context, *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error)
	Clean(context.Context, *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error)
	Install(context.Context, *opv1.InstallRequest) (*opv1.InstallResponse, error)
	Uninstall(context.Context, *opv1.UninstallRequest) (*opv1.InstallResponse, error)
	Run(context.Context, *opv1.RunRequest) (*opv1.RunResponse, error)
	Inspect(context.Context, *opv1.InspectRequest) (*opv1.InspectResponse, error)
	RunSequence(context.Context, *opv1.RunSequenceRequest) (*opv1.RunSequenceResponse, error)
	ModInit(context.Context, *opv1.ModInitRequest) (*opv1.ModInitResponse, error)
	ModAdd(context.Context, *opv1.ModAddRequest) (*opv1.ModAddResponse, error)
	ModRemove(context.Context, *opv1.ModRemoveRequest) (*opv1.ModRemoveResponse, error)
	ModTidy(context.Context, *opv1.ModTidyRequest) (*opv1.ModTidyResponse, error)
	ModPull(context.Context, *opv1.ModPullRequest) (*opv1.ModPullResponse, error)
	ModUpdate(context.Context, *opv1.ModUpdateRequest) (*opv1.ModUpdateResponse, error)
	ModList(context.Context, *opv1.ModListRequest) (*opv1.ModListResponse, error)
	ModGraph(context.Context, *opv1.ModGraphRequest) (*opv1.ModGraphResponse, error)
	Tools(context.Context, *opv1.ToolsRequest) (*opv1.ToolsResponse, error)
	Env(context.Context, *opv1.EnvRequest) (*opv1.EnvResponse, error)
}

type Server struct {
	opv1.UnimplementedOPServiceServer
	handler Handler
}

func New(handler Handler) *Server {
	return &Server{handler: handler}
}

func (s *Server) Discover(ctx context.Context, req *opv1.DiscoverRequest) (*opv1.DiscoverResponse, error) {
	return s.handler.Discover(ctx, req)
}

func (s *Server) Invoke(ctx context.Context, req *opv1.InvokeRequest) (*opv1.InvokeResponse, error) {
	return s.handler.Invoke(ctx, req)
}

func (s *Server) CreateIdentity(ctx context.Context, req *opv1.CreateIdentityRequest) (*opv1.CreateIdentityResponse, error) {
	return s.handler.CreateIdentity(ctx, req)
}

func (s *Server) ListIdentities(ctx context.Context, req *opv1.ListIdentitiesRequest) (*opv1.ListIdentitiesResponse, error) {
	return s.handler.ListIdentities(ctx, req)
}

func (s *Server) ShowIdentity(ctx context.Context, req *opv1.ShowIdentityRequest) (*opv1.ShowIdentityResponse, error) {
	return s.handler.ShowIdentity(ctx, req)
}

func (s *Server) ListTemplates(ctx context.Context, req *opv1.ListTemplatesRequest) (*opv1.ListTemplatesResponse, error) {
	return s.handler.ListTemplates(ctx, req)
}

func (s *Server) GenerateTemplate(ctx context.Context, req *opv1.GenerateTemplateRequest) (*opv1.GenerateTemplateResponse, error) {
	return s.handler.GenerateTemplate(ctx, req)
}

func (s *Server) Check(ctx context.Context, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return s.handler.Check(ctx, req)
}

func (s *Server) Build(ctx context.Context, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return s.handler.Build(ctx, req)
}

func (s *Server) Test(ctx context.Context, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return s.handler.Test(ctx, req)
}

func (s *Server) Clean(ctx context.Context, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return s.handler.Clean(ctx, req)
}

func (s *Server) Install(ctx context.Context, req *opv1.InstallRequest) (*opv1.InstallResponse, error) {
	return s.handler.Install(ctx, req)
}

func (s *Server) Uninstall(ctx context.Context, req *opv1.UninstallRequest) (*opv1.InstallResponse, error) {
	return s.handler.Uninstall(ctx, req)
}

func (s *Server) Run(ctx context.Context, req *opv1.RunRequest) (*opv1.RunResponse, error) {
	return s.handler.Run(ctx, req)
}

func (s *Server) Inspect(ctx context.Context, req *opv1.InspectRequest) (*opv1.InspectResponse, error) {
	return s.handler.Inspect(ctx, req)
}

func (s *Server) RunSequence(ctx context.Context, req *opv1.RunSequenceRequest) (*opv1.RunSequenceResponse, error) {
	return s.handler.RunSequence(ctx, req)
}

func (s *Server) ModInit(ctx context.Context, req *opv1.ModInitRequest) (*opv1.ModInitResponse, error) {
	return s.handler.ModInit(ctx, req)
}

func (s *Server) ModAdd(ctx context.Context, req *opv1.ModAddRequest) (*opv1.ModAddResponse, error) {
	return s.handler.ModAdd(ctx, req)
}

func (s *Server) ModRemove(ctx context.Context, req *opv1.ModRemoveRequest) (*opv1.ModRemoveResponse, error) {
	return s.handler.ModRemove(ctx, req)
}

func (s *Server) ModTidy(ctx context.Context, req *opv1.ModTidyRequest) (*opv1.ModTidyResponse, error) {
	return s.handler.ModTidy(ctx, req)
}

func (s *Server) ModPull(ctx context.Context, req *opv1.ModPullRequest) (*opv1.ModPullResponse, error) {
	return s.handler.ModPull(ctx, req)
}

func (s *Server) ModUpdate(ctx context.Context, req *opv1.ModUpdateRequest) (*opv1.ModUpdateResponse, error) {
	return s.handler.ModUpdate(ctx, req)
}

func (s *Server) ModList(ctx context.Context, req *opv1.ModListRequest) (*opv1.ModListResponse, error) {
	return s.handler.ModList(ctx, req)
}

func (s *Server) ModGraph(ctx context.Context, req *opv1.ModGraphRequest) (*opv1.ModGraphResponse, error) {
	return s.handler.ModGraph(ctx, req)
}

func (s *Server) Tools(ctx context.Context, req *opv1.ToolsRequest) (*opv1.ToolsResponse, error) {
	return s.handler.Tools(ctx, req)
}

func (s *Server) Env(ctx context.Context, req *opv1.EnvRequest) (*opv1.EnvResponse, error) {
	return s.handler.Env(ctx, req)
}

func Register(s *grpc.Server, handler Handler) {
	opv1.RegisterOPServiceServer(s, New(handler))
}
