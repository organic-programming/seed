package transform.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.60.0)",
    comments = "Source: transform/v1/transform.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class TransformServiceGrpc {

  private TransformServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "transform.v1.TransformService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<transform.v1.Transform.TransformRequest,
      transform.v1.Transform.TransformResponse> getTransformMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Transform",
      requestType = transform.v1.Transform.TransformRequest.class,
      responseType = transform.v1.Transform.TransformResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<transform.v1.Transform.TransformRequest,
      transform.v1.Transform.TransformResponse> getTransformMethod() {
    io.grpc.MethodDescriptor<transform.v1.Transform.TransformRequest, transform.v1.Transform.TransformResponse> getTransformMethod;
    if ((getTransformMethod = TransformServiceGrpc.getTransformMethod) == null) {
      synchronized (TransformServiceGrpc.class) {
        if ((getTransformMethod = TransformServiceGrpc.getTransformMethod) == null) {
          TransformServiceGrpc.getTransformMethod = getTransformMethod =
              io.grpc.MethodDescriptor.<transform.v1.Transform.TransformRequest, transform.v1.Transform.TransformResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Transform"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  transform.v1.Transform.TransformRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  transform.v1.Transform.TransformResponse.getDefaultInstance()))
              .setSchemaDescriptor(new TransformServiceMethodDescriptorSupplier("Transform"))
              .build();
        }
      }
    }
    return getTransformMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static TransformServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TransformServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TransformServiceStub>() {
        @java.lang.Override
        public TransformServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TransformServiceStub(channel, callOptions);
        }
      };
    return TransformServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static TransformServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TransformServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TransformServiceBlockingStub>() {
        @java.lang.Override
        public TransformServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TransformServiceBlockingStub(channel, callOptions);
        }
      };
    return TransformServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static TransformServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TransformServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TransformServiceFutureStub>() {
        @java.lang.Override
        public TransformServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TransformServiceFutureStub(channel, callOptions);
        }
      };
    return TransformServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void transform(transform.v1.Transform.TransformRequest request,
        io.grpc.stub.StreamObserver<transform.v1.Transform.TransformResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getTransformMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service TransformService.
   */
  public static abstract class TransformServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return TransformServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service TransformService.
   */
  public static final class TransformServiceStub
      extends io.grpc.stub.AbstractAsyncStub<TransformServiceStub> {
    private TransformServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TransformServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TransformServiceStub(channel, callOptions);
    }

    /**
     */
    public void transform(transform.v1.Transform.TransformRequest request,
        io.grpc.stub.StreamObserver<transform.v1.Transform.TransformResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getTransformMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service TransformService.
   */
  public static final class TransformServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<TransformServiceBlockingStub> {
    private TransformServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TransformServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TransformServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public transform.v1.Transform.TransformResponse transform(transform.v1.Transform.TransformRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getTransformMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service TransformService.
   */
  public static final class TransformServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<TransformServiceFutureStub> {
    private TransformServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TransformServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TransformServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<transform.v1.Transform.TransformResponse> transform(
        transform.v1.Transform.TransformRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getTransformMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_TRANSFORM = 0;

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
        case METHODID_TRANSFORM:
          serviceImpl.transform((transform.v1.Transform.TransformRequest) request,
              (io.grpc.stub.StreamObserver<transform.v1.Transform.TransformResponse>) responseObserver);
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
          getTransformMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              transform.v1.Transform.TransformRequest,
              transform.v1.Transform.TransformResponse>(
                service, METHODID_TRANSFORM)))
        .build();
  }

  private static abstract class TransformServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    TransformServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return transform.v1.Transform.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("TransformService");
    }
  }

  private static final class TransformServiceFileDescriptorSupplier
      extends TransformServiceBaseDescriptorSupplier {
    TransformServiceFileDescriptorSupplier() {}
  }

  private static final class TransformServiceMethodDescriptorSupplier
      extends TransformServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    TransformServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (TransformServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new TransformServiceFileDescriptorSupplier())
              .addMethod(getTransformMethod())
              .build();
        }
      }
    }
    return result;
  }
}
