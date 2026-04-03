package jamesloopsv1

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type JamesLoopsServiceServer interface {
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

type UnimplementedJamesLoopsServiceServer struct{}

func (UnimplementedJamesLoopsServiceServer) Run(context.Context, *RunRequest) (*RunResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Run not implemented")
}

func (UnimplementedJamesLoopsServiceServer) Enqueue(context.Context, *EnqueueRequest) (*EnqueueResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Enqueue not implemented")
}

func (UnimplementedJamesLoopsServiceServer) List(context.Context, *ListRequest) (*ListResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method List not implemented")
}

func (UnimplementedJamesLoopsServiceServer) Status(context.Context, *StatusRequest) (*StatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Status not implemented")
}

func (UnimplementedJamesLoopsServiceServer) Drop(context.Context, *DropRequest) (*DropResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Drop not implemented")
}

func (UnimplementedJamesLoopsServiceServer) Resume(context.Context, *ResumeRequest) (*ResumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Resume not implemented")
}

func (UnimplementedJamesLoopsServiceServer) Skip(context.Context, *SkipRequest) (*SkipResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Skip not implemented")
}

func (UnimplementedJamesLoopsServiceServer) Abort(context.Context, *AbortRequest) (*AbortResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Abort not implemented")
}

func (UnimplementedJamesLoopsServiceServer) ReEnqueue(context.Context, *ReEnqueueRequest) (*ReEnqueueResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ReEnqueue not implemented")
}

func (UnimplementedJamesLoopsServiceServer) Log(context.Context, *LogRequest) (*LogResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Log not implemented")
}

func RegisterJamesLoopsServiceServer(s grpc.ServiceRegistrar, srv JamesLoopsServiceServer) {
	s.RegisterService(&JamesLoopsService_ServiceDesc, srv)
}

var JamesLoopsService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "jamesloops.v1.JamesLoopsService",
	HandlerType: (*JamesLoopsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Run", Handler: _JamesLoopsService_Run_Handler},
		{MethodName: "Enqueue", Handler: _JamesLoopsService_Enqueue_Handler},
		{MethodName: "List", Handler: _JamesLoopsService_List_Handler},
		{MethodName: "Status", Handler: _JamesLoopsService_Status_Handler},
		{MethodName: "Drop", Handler: _JamesLoopsService_Drop_Handler},
		{MethodName: "Resume", Handler: _JamesLoopsService_Resume_Handler},
		{MethodName: "Skip", Handler: _JamesLoopsService_Skip_Handler},
		{MethodName: "Abort", Handler: _JamesLoopsService_Abort_Handler},
		{MethodName: "ReEnqueue", Handler: _JamesLoopsService_ReEnqueue_Handler},
		{MethodName: "Log", Handler: _JamesLoopsService_Log_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/v1/holon.proto",
}

func _JamesLoopsService_Run_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).Run(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/Run"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).Run(ctx, req.(*RunRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_Enqueue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EnqueueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).Enqueue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/Enqueue"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).Enqueue(ctx, req.(*EnqueueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/List"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).List(ctx, req.(*ListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_Status_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).Status(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/Status"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).Status(ctx, req.(*StatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_Drop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DropRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).Drop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/Drop"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).Drop(ctx, req.(*DropRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_Resume_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ResumeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).Resume(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/Resume"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).Resume(ctx, req.(*ResumeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_Skip_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SkipRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).Skip(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/Skip"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).Skip(ctx, req.(*SkipRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_Abort_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AbortRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).Abort(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/Abort"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).Abort(ctx, req.(*AbortRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_ReEnqueue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReEnqueueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).ReEnqueue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/ReEnqueue"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).ReEnqueue(ctx, req.(*ReEnqueueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JamesLoopsService_Log_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JamesLoopsServiceServer).Log(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/jamesloops.v1.JamesLoopsService/Log"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JamesLoopsServiceServer).Log(ctx, req.(*LogRequest))
	}
	return interceptor(ctx, in, info, handler)
}
