package greeting.v1

import greeting.v1.GreetingServiceGrpc.getServiceDescriptor
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

/**
 * Holder for Kotlin coroutine-based client and server APIs for greeting.v1.GreetingService.
 */
public object GreetingServiceGrpcKt {
  public const val SERVICE_NAME: String = GreetingServiceGrpc.SERVICE_NAME

  @JvmStatic
  public val serviceDescriptor: ServiceDescriptor
    get() = getServiceDescriptor()

  public val listLanguagesMethod:
      MethodDescriptor<Greeting.ListLanguagesRequest, Greeting.ListLanguagesResponse>
    @JvmStatic
    get() = GreetingServiceGrpc.getListLanguagesMethod()

  public val sayHelloMethod: MethodDescriptor<Greeting.SayHelloRequest, Greeting.SayHelloResponse>
    @JvmStatic
    get() = GreetingServiceGrpc.getSayHelloMethod()

  /**
   * A stub for issuing RPCs to a(n) greeting.v1.GreetingService service as suspending coroutines.
   */
  @StubFor(GreetingServiceGrpc::class)
  public class GreetingServiceCoroutineStub @JvmOverloads constructor(
    channel: Channel,
    callOptions: CallOptions = DEFAULT,
  ) : AbstractCoroutineStub<GreetingServiceCoroutineStub>(channel, callOptions) {
    override fun build(channel: Channel, callOptions: CallOptions): GreetingServiceCoroutineStub =
        GreetingServiceCoroutineStub(channel, callOptions)

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
    public suspend fun listLanguages(request: Greeting.ListLanguagesRequest, headers: Metadata =
        Metadata()): Greeting.ListLanguagesResponse = unaryRpc(
      channel,
      GreetingServiceGrpc.getListLanguagesMethod(),
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
    public suspend fun sayHello(request: Greeting.SayHelloRequest, headers: Metadata = Metadata()):
        Greeting.SayHelloResponse = unaryRpc(
      channel,
      GreetingServiceGrpc.getSayHelloMethod(),
      request,
      callOptions,
      headers
    )
  }

  /**
   * Skeletal implementation of the greeting.v1.GreetingService service based on Kotlin coroutines.
   */
  public abstract class GreetingServiceCoroutineImplBase(
    coroutineContext: CoroutineContext = EmptyCoroutineContext,
  ) : AbstractCoroutineServerImpl(coroutineContext) {
    /**
     * Returns the response to an RPC for greeting.v1.GreetingService.ListLanguages.
     *
     * If this method fails with a [StatusException], the RPC will fail with the corresponding
     * [io.grpc.Status].  If this method fails with a [java.util.concurrent.CancellationException],
     * the RPC will fail
     * with status `Status.CANCELLED`.  If this method fails for any other reason, the RPC will
     * fail with `Status.UNKNOWN` with the exception as a cause.
     *
     * @param request The request from the client.
     */
    public open suspend fun listLanguages(request: Greeting.ListLanguagesRequest):
        Greeting.ListLanguagesResponse = throw
        StatusException(UNIMPLEMENTED.withDescription("Method greeting.v1.GreetingService.ListLanguages is unimplemented"))

    /**
     * Returns the response to an RPC for greeting.v1.GreetingService.SayHello.
     *
     * If this method fails with a [StatusException], the RPC will fail with the corresponding
     * [io.grpc.Status].  If this method fails with a [java.util.concurrent.CancellationException],
     * the RPC will fail
     * with status `Status.CANCELLED`.  If this method fails for any other reason, the RPC will
     * fail with `Status.UNKNOWN` with the exception as a cause.
     *
     * @param request The request from the client.
     */
    public open suspend fun sayHello(request: Greeting.SayHelloRequest): Greeting.SayHelloResponse =
        throw
        StatusException(UNIMPLEMENTED.withDescription("Method greeting.v1.GreetingService.SayHello is unimplemented"))

    final override fun bindService(): ServerServiceDefinition = builder(getServiceDescriptor())
      .addMethod(unaryServerMethodDefinition(
      context = this.context,
      descriptor = GreetingServiceGrpc.getListLanguagesMethod(),
      implementation = ::listLanguages
    ))
      .addMethod(unaryServerMethodDefinition(
      context = this.context,
      descriptor = GreetingServiceGrpc.getSayHelloMethod(),
      implementation = ::sayHello
    )).build()
  }
}
