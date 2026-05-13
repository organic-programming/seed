package relay.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class RelayServiceGrpc {

  private RelayServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "relay.v1.RelayService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<relay.v1.Relay.TickRequest,
      relay.v1.Relay.TickResponse> getTickMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Tick",
      requestType = relay.v1.Relay.TickRequest.class,
      responseType = relay.v1.Relay.TickResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<relay.v1.Relay.TickRequest,
      relay.v1.Relay.TickResponse> getTickMethod() {
    io.grpc.MethodDescriptor<relay.v1.Relay.TickRequest, relay.v1.Relay.TickResponse> getTickMethod;
    if ((getTickMethod = RelayServiceGrpc.getTickMethod) == null) {
      synchronized (RelayServiceGrpc.class) {
        if ((getTickMethod = RelayServiceGrpc.getTickMethod) == null) {
          RelayServiceGrpc.getTickMethod = getTickMethod =
              io.grpc.MethodDescriptor.<relay.v1.Relay.TickRequest, relay.v1.Relay.TickResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Tick"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  relay.v1.Relay.TickRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  relay.v1.Relay.TickResponse.getDefaultInstance()))
              .setSchemaDescriptor(new RelayServiceMethodDescriptorSupplier("Tick"))
              .build();
        }
      }
    }
    return getTickMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static RelayServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<RelayServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<RelayServiceStub>() {
        @java.lang.Override
        public RelayServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new RelayServiceStub(channel, callOptions);
        }
      };
    return RelayServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static RelayServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<RelayServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<RelayServiceBlockingV2Stub>() {
        @java.lang.Override
        public RelayServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new RelayServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return RelayServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static RelayServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<RelayServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<RelayServiceBlockingStub>() {
        @java.lang.Override
        public RelayServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new RelayServiceBlockingStub(channel, callOptions);
        }
      };
    return RelayServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static RelayServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<RelayServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<RelayServiceFutureStub>() {
        @java.lang.Override
        public RelayServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new RelayServiceFutureStub(channel, callOptions);
        }
      };
    return RelayServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     * <pre>
     * Tick: emit one log + increment one metric counter at the receiver.
     * Used to test cross-holon observability relay: send a Tick to a leaf
     * holon and verify the log propagates up the MemberEndpoints chain.
     * Metrics are NOT relayed by the SDK - they are exposed locally and
     * verified at each node directly.
     * </pre>
     */
    default void tick(relay.v1.Relay.TickRequest request,
        io.grpc.stub.StreamObserver<relay.v1.Relay.TickResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getTickMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service RelayService.
   */
  public static abstract class RelayServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return RelayServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service RelayService.
   */
  public static final class RelayServiceStub
      extends io.grpc.stub.AbstractAsyncStub<RelayServiceStub> {
    private RelayServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected RelayServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new RelayServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Tick: emit one log + increment one metric counter at the receiver.
     * Used to test cross-holon observability relay: send a Tick to a leaf
     * holon and verify the log propagates up the MemberEndpoints chain.
     * Metrics are NOT relayed by the SDK - they are exposed locally and
     * verified at each node directly.
     * </pre>
     */
    public void tick(relay.v1.Relay.TickRequest request,
        io.grpc.stub.StreamObserver<relay.v1.Relay.TickResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getTickMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service RelayService.
   */
  public static final class RelayServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<RelayServiceBlockingV2Stub> {
    private RelayServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected RelayServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new RelayServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Tick: emit one log + increment one metric counter at the receiver.
     * Used to test cross-holon observability relay: send a Tick to a leaf
     * holon and verify the log propagates up the MemberEndpoints chain.
     * Metrics are NOT relayed by the SDK - they are exposed locally and
     * verified at each node directly.
     * </pre>
     */
    public relay.v1.Relay.TickResponse tick(relay.v1.Relay.TickRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getTickMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service RelayService.
   */
  public static final class RelayServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<RelayServiceBlockingStub> {
    private RelayServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected RelayServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new RelayServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Tick: emit one log + increment one metric counter at the receiver.
     * Used to test cross-holon observability relay: send a Tick to a leaf
     * holon and verify the log propagates up the MemberEndpoints chain.
     * Metrics are NOT relayed by the SDK - they are exposed locally and
     * verified at each node directly.
     * </pre>
     */
    public relay.v1.Relay.TickResponse tick(relay.v1.Relay.TickRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getTickMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service RelayService.
   */
  public static final class RelayServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<RelayServiceFutureStub> {
    private RelayServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected RelayServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new RelayServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Tick: emit one log + increment one metric counter at the receiver.
     * Used to test cross-holon observability relay: send a Tick to a leaf
     * holon and verify the log propagates up the MemberEndpoints chain.
     * Metrics are NOT relayed by the SDK - they are exposed locally and
     * verified at each node directly.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<relay.v1.Relay.TickResponse> tick(
        relay.v1.Relay.TickRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getTickMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_TICK = 0;

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
        case METHODID_TICK:
          serviceImpl.tick((relay.v1.Relay.TickRequest) request,
              (io.grpc.stub.StreamObserver<relay.v1.Relay.TickResponse>) responseObserver);
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
          getTickMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              relay.v1.Relay.TickRequest,
              relay.v1.Relay.TickResponse>(
                service, METHODID_TICK)))
        .build();
  }

  private static abstract class RelayServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    RelayServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return relay.v1.Relay.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("RelayService");
    }
  }

  private static final class RelayServiceFileDescriptorSupplier
      extends RelayServiceBaseDescriptorSupplier {
    RelayServiceFileDescriptorSupplier() {}
  }

  private static final class RelayServiceMethodDescriptorSupplier
      extends RelayServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    RelayServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (RelayServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new RelayServiceFileDescriptorSupplier())
              .addMethod(getTickMethod())
              .build();
        }
      }
    }
    return result;
  }
}
