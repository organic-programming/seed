import GRPC
import NIOCore
import SwiftProtobuf

// GreetingAppServiceProvider implements the COAX domain surface
// (greeting.v1.GreetingAppService) for the Gabriel Greeting App.
// These are the same actions a human performs through the UI,
// expressed as RPCs an agent can call equivalently.
public final class GreetingAppServiceProvider: CallHandlerProvider, @unchecked Sendable {
  public let serviceName: Substring = "greeting.v1.GreetingAppService"

  private let holon: GreetingHolonManager

  public init(holon: GreetingHolonManager) {
    self.holon = holon
  }

  public func handle(method name: Substring, context: CallHandlerContext)
    -> GRPCServerHandlerProtocol?
  {
    switch name {
    case "SelectHolon":
      return UnaryServerHandler(
        context: context,
        requestDeserializer: ProtobufDeserializer<Greeting_V1_SelectHolonRequest>(),
        responseSerializer: ProtobufSerializer<Greeting_V1_SelectHolonResponse>(),
        interceptors: [],
        userFunction: selectHolon(request:context:)
      )
    case "SelectTransport":
      return UnaryServerHandler(
        context: context,
        requestDeserializer: ProtobufDeserializer<Greeting_V1_SelectTransportRequest>(),
        responseSerializer: ProtobufSerializer<Greeting_V1_SelectTransportResponse>(),
        interceptors: [],
        userFunction: selectTransport(request:context:)
      )
    case "SelectLanguage":
      return UnaryServerHandler(
        context: context,
        requestDeserializer: ProtobufDeserializer<Greeting_V1_SelectLanguageRequest>(),
        responseSerializer: ProtobufSerializer<Greeting_V1_SelectLanguageResponse>(),
        interceptors: [],
        userFunction: selectLanguage(request:context:)
      )
    case "Greet":
      return UnaryServerHandler(
        context: context,
        requestDeserializer: ProtobufDeserializer<Greeting_V1_GreetRequest>(),
        responseSerializer: ProtobufSerializer<Greeting_V1_GreetResponse>(),
        interceptors: [],
        userFunction: greet(request:context:)
      )
    default:
      return nil
    }
  }

  // MARK: - RPC Implementations

  private func grpcStatus(for error: Error) -> GRPCStatus {
    if let status = error as? GRPCStatus {
      return status
    }
    if let selectionError = error as? GreetingSelectionError {
      switch selectionError {
      case .holonNotFound:
        return GRPCStatus(code: .notFound, message: selectionError.localizedDescription)
      case .unsupportedTransport, .unsupportedLanguage, .noLanguageSelected:
        return GRPCStatus(code: .invalidArgument, message: selectionError.localizedDescription)
      case .noHolonsFound:
        return GRPCStatus(code: .unavailable, message: selectionError.localizedDescription)
      }
    }
    if let holonError = error as? HolonError {
      return GRPCStatus(code: .unavailable, message: holonError.localizedDescription)
    }
    return GRPCStatus(code: .unavailable, message: error.localizedDescription)
  }

  private func selectHolon(
    request: Greeting_V1_SelectHolonRequest,
    context: StatusOnlyCallContext
  ) -> EventLoopFuture<Greeting_V1_SelectHolonResponse> {
    let promise = context.eventLoop.makePromise(of: Greeting_V1_SelectHolonResponse.self)
    Task { @MainActor [holon] in
      do {
        let identity = try await holon.selectHolon(slug: request.slug, greetAfterLoad: true)
        var response = Greeting_V1_SelectHolonResponse()
        response.slug = identity.slug
        response.displayName = identity.displayName
        promise.succeed(response)
      } catch {
        promise.fail(grpcStatus(for: error))
      }
    }
    return promise.futureResult
  }

  private func selectTransport(
    request: Greeting_V1_SelectTransportRequest,
    context: StatusOnlyCallContext
  ) -> EventLoopFuture<Greeting_V1_SelectTransportResponse> {
    let promise = context.eventLoop.makePromise(of: Greeting_V1_SelectTransportResponse.self)
    Task { @MainActor [holon] in
      do {
        let transport = try await holon.selectTransport(request.transport, greetAfterLoad: true)
        var response = Greeting_V1_SelectTransportResponse()
        response.transport = transport
        promise.succeed(response)
      } catch {
        promise.fail(grpcStatus(for: error))
      }
    }
    return promise.futureResult
  }

  private func selectLanguage(
    request: Greeting_V1_SelectLanguageRequest,
    context: StatusOnlyCallContext
  ) -> EventLoopFuture<Greeting_V1_SelectLanguageResponse> {
    let promise = context.eventLoop.makePromise(of: Greeting_V1_SelectLanguageResponse.self)
    Task { @MainActor [holon] in
      do {
        let code = try await holon.selectLanguageAndGreet(request.code)
        var response = Greeting_V1_SelectLanguageResponse()
        response.code = code
        promise.succeed(response)
      } catch {
        promise.fail(grpcStatus(for: error))
      }
    }
    return promise.futureResult
  }

  private func greet(
    request: Greeting_V1_GreetRequest,
    context: StatusOnlyCallContext
  ) -> EventLoopFuture<Greeting_V1_GreetResponse> {
    let promise = context.eventLoop.makePromise(of: Greeting_V1_GreetResponse.self)
    Task { @MainActor [holon] in
      do {
        let greeting = try await holon.greetCurrentSelection(
          name: request.name.isEmpty ? nil : request.name,
          langCode: request.langCode.isEmpty ? nil : request.langCode
        )
        var response = Greeting_V1_GreetResponse()
        response.greeting = greeting
        promise.succeed(response)
      } catch {
        promise.fail(grpcStatus(for: error))
      }
    }
    return promise.futureResult
  }
}
