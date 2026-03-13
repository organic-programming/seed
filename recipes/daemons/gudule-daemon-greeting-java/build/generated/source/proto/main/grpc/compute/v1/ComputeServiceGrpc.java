package compute.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.60.0)",
    comments = "Source: compute/v1/compute.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class ComputeServiceGrpc {

  private ComputeServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "compute.v1.ComputeService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<compute.v1.Compute.ComputeRequest,
      compute.v1.Compute.ComputeResponse> getComputeMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Compute",
      requestType = compute.v1.Compute.ComputeRequest.class,
      responseType = compute.v1.Compute.ComputeResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<compute.v1.Compute.ComputeRequest,
      compute.v1.Compute.ComputeResponse> getComputeMethod() {
    io.grpc.MethodDescriptor<compute.v1.Compute.ComputeRequest, compute.v1.Compute.ComputeResponse> getComputeMethod;
    if ((getComputeMethod = ComputeServiceGrpc.getComputeMethod) == null) {
      synchronized (ComputeServiceGrpc.class) {
        if ((getComputeMethod = ComputeServiceGrpc.getComputeMethod) == null) {
          ComputeServiceGrpc.getComputeMethod = getComputeMethod =
              io.grpc.MethodDescriptor.<compute.v1.Compute.ComputeRequest, compute.v1.Compute.ComputeResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Compute"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  compute.v1.Compute.ComputeRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  compute.v1.Compute.ComputeResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ComputeServiceMethodDescriptorSupplier("Compute"))
              .build();
        }
      }
    }
    return getComputeMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ComputeServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ComputeServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ComputeServiceStub>() {
        @java.lang.Override
        public ComputeServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ComputeServiceStub(channel, callOptions);
        }
      };
    return ComputeServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ComputeServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ComputeServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ComputeServiceBlockingStub>() {
        @java.lang.Override
        public ComputeServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ComputeServiceBlockingStub(channel, callOptions);
        }
      };
    return ComputeServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ComputeServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ComputeServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ComputeServiceFutureStub>() {
        @java.lang.Override
        public ComputeServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ComputeServiceFutureStub(channel, callOptions);
        }
      };
    return ComputeServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void compute(compute.v1.Compute.ComputeRequest request,
        io.grpc.stub.StreamObserver<compute.v1.Compute.ComputeResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getComputeMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ComputeService.
   */
  public static abstract class ComputeServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ComputeServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ComputeService.
   */
  public static final class ComputeServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ComputeServiceStub> {
    private ComputeServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ComputeServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ComputeServiceStub(channel, callOptions);
    }

    /**
     */
    public void compute(compute.v1.Compute.ComputeRequest request,
        io.grpc.stub.StreamObserver<compute.v1.Compute.ComputeResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getComputeMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ComputeService.
   */
  public static final class ComputeServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ComputeServiceBlockingStub> {
    private ComputeServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ComputeServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ComputeServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public compute.v1.Compute.ComputeResponse compute(compute.v1.Compute.ComputeRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getComputeMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ComputeService.
   */
  public static final class ComputeServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ComputeServiceFutureStub> {
    private ComputeServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ComputeServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ComputeServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<compute.v1.Compute.ComputeResponse> compute(
        compute.v1.Compute.ComputeRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getComputeMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_COMPUTE = 0;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_COMPUTE:
          serviceImpl.compute((compute.v1.Compute.ComputeRequest) request,
              (io.grpc.stub.StreamObserver<compute.v1.Compute.ComputeResponse>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getComputeMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              compute.v1.Compute.ComputeRequest,
              compute.v1.Compute.ComputeResponse>(
                service, METHODID_COMPUTE)))
        .build();
  }

  private static abstract class ComputeServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ComputeServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return compute.v1.Compute.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ComputeService");
    }
  }

  private static final class ComputeServiceFileDescriptorSupplier
      extends ComputeServiceBaseDescriptorSupplier {
    ComputeServiceFileDescriptorSupplier() {}
  }

  private static final class ComputeServiceMethodDescriptorSupplier
      extends ComputeServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ComputeServiceMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (ComputeServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ComputeServiceFileDescriptorSupplier())
              .addMethod(getComputeMethod())
              .build();
        }
      }
    }
    return result;
  }
}
