package codexloopsv1

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CodexLoopsServiceServer interface {
	Run(context.Context, *RunRequest) (*RunResponse, error)
	Enqueue(context.Context, *EnqueueRequest) (*EnqueueResponse, error)
	List(context.Context, *ListRequest) (*ListResponse, error)
	Status(context.Context, *StatusRequest) (*StatusResponse, error)
	Drop(context.Context, *DropRequest) (*DropResponse, error)
	Resume(context.Context, *ResumeRequest) (*ResumeResponse, error)
	Skip(context.Context, *SkipRequest) (*SkipResponse, error)
	Abort(context.Context, *AbortRequest) (*AbortResponse, error)
	ReEnqueue(context.Context, *ReEnqueueRequest) (*ReEnqueueResponse, error)
	Log(context.Context, *LogRequest) (*LogResponse, error)
}

type UnimplementedCodexLoopsServiceServer struct{}

func (UnimplementedCodexLoopsServiceServer) Run(context.Context, *RunRequest) (*RunResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Run not implemented")
}

func (UnimplementedCodexLoopsServiceServer) Enqueue(context.Context, *EnqueueRequest) (*EnqueueResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Enqueue not implemented")
}

func (UnimplementedCodexLoopsServiceServer) List(context.Context, *ListRequest) (*ListResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method List not implemented")
}

func (UnimplementedCodexLoopsServiceServer) Status(context.Context, *StatusRequest) (*StatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Status not implemented")
}

func (UnimplementedCodexLoopsServiceServer) Drop(context.Context, *DropRequest) (*DropResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Drop not implemented")
}

func (UnimplementedCodexLoopsServiceServer) Resume(context.Context, *ResumeRequest) (*ResumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Resume not implemented")
}

func (UnimplementedCodexLoopsServiceServer) Skip(context.Context, *SkipRequest) (*SkipResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Skip not implemented")
}

func (UnimplementedCodexLoopsServiceServer) Abort(context.Context, *AbortRequest) (*AbortResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Abort not implemented")
}

func (UnimplementedCodexLoopsServiceServer) ReEnqueue(context.Context, *ReEnqueueRequest) (*ReEnqueueResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ReEnqueue not implemented")
}

func (UnimplementedCodexLoopsServiceServer) Log(context.Context, *LogRequest) (*LogResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Log not implemented")
}

func RegisterCodexLoopsServiceServer(s grpc.ServiceRegistrar, srv CodexLoopsServiceServer) {
	s.RegisterService(&CodexLoopsService_ServiceDesc, srv)
}

var CodexLoopsService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "codexloops.v1.CodexLoopsService",
	HandlerType: (*CodexLoopsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Run", Handler: _CodexLoopsService_Run_Handler},
		{MethodName: "Enqueue", Handler: _CodexLoopsService_Enqueue_Handler},
		{MethodName: "List", Handler: _CodexLoopsService_List_Handler},
		{MethodName: "Status", Handler: _CodexLoopsService_Status_Handler},
		{MethodName: "Drop", Handler: _CodexLoopsService_Drop_Handler},
		{MethodName: "Resume", Handler: _CodexLoopsService_Resume_Handler},
		{MethodName: "Skip", Handler: _CodexLoopsService_Skip_Handler},
		{MethodName: "Abort", Handler: _CodexLoopsService_Abort_Handler},
		{MethodName: "ReEnqueue", Handler: _CodexLoopsService_ReEnqueue_Handler},
		{MethodName: "Log", Handler: _CodexLoopsService_Log_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/v1/holon.proto",
}

func _CodexLoopsService_Run_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).Run(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/Run"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).Run(ctx, req.(*RunRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_Enqueue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EnqueueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).Enqueue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/Enqueue"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).Enqueue(ctx, req.(*EnqueueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/List"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).List(ctx, req.(*ListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_Status_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).Status(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/Status"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).Status(ctx, req.(*StatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_Drop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DropRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).Drop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/Drop"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).Drop(ctx, req.(*DropRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_Resume_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ResumeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).Resume(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/Resume"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).Resume(ctx, req.(*ResumeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_Skip_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SkipRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).Skip(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/Skip"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).Skip(ctx, req.(*SkipRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_Abort_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AbortRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).Abort(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/Abort"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).Abort(ctx, req.(*AbortRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_ReEnqueue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReEnqueueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).ReEnqueue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/ReEnqueue"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).ReEnqueue(ctx, req.(*ReEnqueueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CodexLoopsService_Log_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CodexLoopsServiceServer).Log(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/codexloops.v1.CodexLoopsService/Log"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CodexLoopsServiceServer).Log(ctx, req.(*LogRequest))
	}
	return interceptor(ctx, in, info, handler)
}
