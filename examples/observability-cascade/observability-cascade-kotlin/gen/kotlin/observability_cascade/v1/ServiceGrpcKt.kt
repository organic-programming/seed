package observability_cascade.v1

import io.grpc.CallOptions
import io.grpc.CallOptions.DEFAULT
import io.grpc.Channel
import io.grpc.Metadata
import io.grpc.MethodDescriptor
import io.grpc.ServerServiceDefinition
import io.grpc.ServerServiceDefinition.builder
import io.grpc.ServiceDescriptor
import io.grpc.Status.UNIMPLEMENTED
import io.grpc.StatusException
import io.grpc.kotlin.AbstractCoroutineServerImpl
import io.grpc.kotlin.AbstractCoroutineStub
import io.grpc.kotlin.ClientCalls.unaryRpc
import io.grpc.kotlin.ServerCalls.unaryServerMethodDefinition
import io.grpc.kotlin.StubFor
import kotlin.String
import kotlin.coroutines.CoroutineContext
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.jvm.JvmOverloads
import kotlin.jvm.JvmStatic
import observability_cascade.v1.ObservabilityCascadeServiceGrpc.getServiceDescriptor

/**
 * Holder for Kotlin coroutine-based client and server APIs for
 * observability_cascade.v1.ObservabilityCascadeService.
 */
public object ObservabilityCascadeServiceGrpcKt {
  public const val SERVICE_NAME: String = ObservabilityCascadeServiceGrpc.SERVICE_NAME

  @JvmStatic
  public val serviceDescriptor: ServiceDescriptor
    get() = getServiceDescriptor()

  public val runDefaultMethod: MethodDescriptor<Service.RunRequest, Service.CascadeReport>
    @JvmStatic
    get() = ObservabilityCascadeServiceGrpc.getRunDefaultMethod()

  public val runLiveStreamMethod: MethodDescriptor<Service.RunRequest, Service.CascadeReport>
    @JvmStatic
    get() = ObservabilityCascadeServiceGrpc.getRunLiveStreamMethod()

  public val runMultiPatternMethod: MethodDescriptor<Service.RunRequest, Service.MultiPatternReport>
    @JvmStatic
    get() = ObservabilityCascadeServiceGrpc.getRunMultiPatternMethod()

  /**
   * A stub for issuing RPCs to a(n) observability_cascade.v1.ObservabilityCascadeService service as
   * suspending coroutines.
   */
  @StubFor(ObservabilityCascadeServiceGrpc::class)
  public class ObservabilityCascadeServiceCoroutineStub @JvmOverloads constructor(
    channel: Channel,
    callOptions: CallOptions = DEFAULT,
  ) : AbstractCoroutineStub<ObservabilityCascadeServiceCoroutineStub>(channel, callOptions) {
    override fun build(channel: Channel, callOptions: CallOptions):
        ObservabilityCascadeServiceCoroutineStub = ObservabilityCascadeServiceCoroutineStub(channel,
        callOptions)

    /**
     * Executes this RPC and returns the response message, suspending until the RPC completes
     * with [`Status.OK`][io.grpc.Status].  If the RPC completes with another status, a
     * corresponding
     * [StatusException] is thrown.  If this coroutine is cancelled, the RPC is also cancelled
     * with the corresponding exception as a cause.
     *
     * @param request The request message to send to the server.
     *
     * @param headers Metadata to attach to the request.  Most users will not need this.
     *
     * @return The single response from the server.
     */
    public suspend fun runDefault(request: Service.RunRequest, headers: Metadata = Metadata()):
        Service.CascadeReport = unaryRpc(
      channel,
      ObservabilityCascadeServiceGrpc.getRunDefaultMethod(),
      request,
      callOptions,
      headers
    )

    /**
     * Executes this RPC and returns the response message, suspending until the RPC completes
     * with [`Status.OK`][io.grpc.Status].  If the RPC completes with another status, a
     * corresponding
     * [StatusException] is thrown.  If this coroutine is cancelled, the RPC is also cancelled
     * with the corresponding exception as a cause.
     *
     * @param request The request message to send to the server.
     *
     * @param headers Metadata to attach to the request.  Most users will not need this.
     *
     * @return The single response from the server.
     */
    public suspend fun runLiveStream(request: Service.RunRequest, headers: Metadata = Metadata()):
        Service.CascadeReport = unaryRpc(
      channel,
      ObservabilityCascadeServiceGrpc.getRunLiveStreamMethod(),
      request,
      callOptions,
      headers
    )

    /**
     * Executes this RPC and returns the response message, suspending until the RPC completes
     * with [`Status.OK`][io.grpc.Status].  If the RPC completes with another status, a
     * corresponding
     * [StatusException] is thrown.  If this coroutine is cancelled, the RPC is also cancelled
     * with the corresponding exception as a cause.
     *
     * @param request The request message to send to the server.
     *
     * @param headers Metadata to attach to the request.  Most users will not need this.
     *
     * @return The single response from the server.
     */
    public suspend fun runMultiPattern(request: Service.RunRequest, headers: Metadata = Metadata()):
        Service.MultiPatternReport = unaryRpc(
      channel,
      ObservabilityCascadeServiceGrpc.getRunMultiPatternMethod(),
      request,
      callOptions,
      headers
    )
  }

  /**
   * Skeletal implementation of the observability_cascade.v1.ObservabilityCascadeService service
   * based on Kotlin coroutines.
   */
  public abstract class ObservabilityCascadeServiceCoroutineImplBase(
    coroutineContext: CoroutineContext = EmptyCoroutineContext,
  ) : AbstractCoroutineServerImpl(coroutineContext) {
    /**
     * Returns the response to an RPC for
     * observability_cascade.v1.ObservabilityCascadeService.RunDefault.
     *
     * If this method fails with a [StatusException], the RPC will fail with the corresponding
     * [io.grpc.Status].  If this method fails with a [java.util.concurrent.CancellationException],
     * the RPC will fail
     * with status `Status.CANCELLED`.  If this method fails for any other reason, the RPC will
     * fail with `Status.UNKNOWN` with the exception as a cause.
     *
     * @param request The request from the client.
     */
    public open suspend fun runDefault(request: Service.RunRequest): Service.CascadeReport = throw
        StatusException(UNIMPLEMENTED.withDescription("Method observability_cascade.v1.ObservabilityCascadeService.RunDefault is unimplemented"))

    /**
     * Returns the response to an RPC for
     * observability_cascade.v1.ObservabilityCascadeService.RunLiveStream.
     *
     * If this method fails with a [StatusException], the RPC will fail with the corresponding
     * [io.grpc.Status].  If this method fails with a [java.util.concurrent.CancellationException],
     * the RPC will fail
     * with status `Status.CANCELLED`.  If this method fails for any other reason, the RPC will
     * fail with `Status.UNKNOWN` with the exception as a cause.
     *
     * @param request The request from the client.
     */
    public open suspend fun runLiveStream(request: Service.RunRequest): Service.CascadeReport =
        throw
        StatusException(UNIMPLEMENTED.withDescription("Method observability_cascade.v1.ObservabilityCascadeService.RunLiveStream is unimplemented"))

    /**
     * Returns the response to an RPC for
     * observability_cascade.v1.ObservabilityCascadeService.RunMultiPattern.
     *
     * If this method fails with a [StatusException], the RPC will fail with the corresponding
     * [io.grpc.Status].  If this method fails with a [java.util.concurrent.CancellationException],
     * the RPC will fail
     * with status `Status.CANCELLED`.  If this method fails for any other reason, the RPC will
     * fail with `Status.UNKNOWN` with the exception as a cause.
     *
     * @param request The request from the client.
     */
    public open suspend fun runMultiPattern(request: Service.RunRequest): Service.MultiPatternReport
        = throw
        StatusException(UNIMPLEMENTED.withDescription("Method observability_cascade.v1.ObservabilityCascadeService.RunMultiPattern is unimplemented"))

    final override fun bindService(): ServerServiceDefinition = builder(getServiceDescriptor())
      .addMethod(unaryServerMethodDefinition(
      context = this.context,
      descriptor = ObservabilityCascadeServiceGrpc.getRunDefaultMethod(),
      implementation = ::runDefault
    ))
      .addMethod(unaryServerMethodDefinition(
      context = this.context,
      descriptor = ObservabilityCascadeServiceGrpc.getRunLiveStreamMethod(),
      implementation = ::runLiveStream
    ))
      .addMethod(unaryServerMethodDefinition(
      context = this.context,
      descriptor = ObservabilityCascadeServiceGrpc.getRunMultiPatternMethod(),
      implementation = ::runMultiPattern
    )).build()
  }
}
