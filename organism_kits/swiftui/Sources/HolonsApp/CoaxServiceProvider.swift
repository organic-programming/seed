import Foundation
import GRPC
import Holons
import NIOCore
import SwiftProtobuf

public final class CoaxRpcServiceProvider: CallHandlerProvider, @unchecked Sendable {
    public let serviceName: Substring = "holons.v1.CoaxService"

    private let holonManager: any HolonManager
    private let turnOffCoax: @MainActor @Sendable () -> Void

    public init(
        holonManager: any HolonManager,
        turnOffCoax: @escaping @MainActor @Sendable () -> Void
    ) {
        self.holonManager = holonManager
        self.turnOffCoax = turnOffCoax
    }

    @available(*, deprecated, message: "Use init(holonManager:coaxManager:)")
    public convenience init(holonManager: any HolonManager, coaxManager: CoaxManager) {
        self.init(holonManager: holonManager) {
            coaxManager.turnOffAfterRpc()
        }
    }

    @available(*, deprecated, message: "Use init(holonManager:turnOffCoax:)")
    public convenience init(organism: any OrganismState, coaxServer: CoaxServer) {
        self.init(holonManager: organism) {
            coaxServer.turnOffAfterRpc()
        }
    }

    public func handle(method name: Substring, context: CallHandlerContext) -> GRPCServerHandlerProtocol? {
        switch name {
        case "ListMembers":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_ListMembersRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_ListMembersResponse>(),
                interceptors: [],
                userFunction: listMembers(request:context:)
            )
        case "MemberStatus":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_MemberStatusRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_MemberStatusResponse>(),
                interceptors: [],
                userFunction: memberStatus(request:context:)
            )
        case "ConnectMember":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_ConnectMemberRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_ConnectMemberResponse>(),
                interceptors: [],
                userFunction: connectMember(request:context:)
            )
        case "DisconnectMember":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_DisconnectMemberRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_DisconnectMemberResponse>(),
                interceptors: [],
                userFunction: disconnectMember(request:context:)
            )
        case "Tell":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_TellRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_TellResponse>(),
                interceptors: [],
                userFunction: tell(request:context:)
            )
        case "TurnOffCoax":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_TurnOffCoaxRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_TurnOffCoaxResponse>(),
                interceptors: [],
                userFunction: turnOffCoax(request:context:)
            )
        default:
            return nil
        }
    }

    private func listMembers(
        request: Holons_V1_ListMembersRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_ListMembersResponse> {
        _ = request
        let promise = context.eventLoop.makePromise(of: Holons_V1_ListMembersResponse.self)
        Task { @MainActor [holonManager] in
            var response = Holons_V1_ListMembersResponse()
            response.members = await holonManager.listMembers().map(memberInfo(for:))
            promise.succeed(response)
        }
        return promise.futureResult
    }

    private func memberStatus(
        request: Holons_V1_MemberStatusRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_MemberStatusResponse> {
        let promise = context.eventLoop.makePromise(of: Holons_V1_MemberStatusResponse.self)
        Task { @MainActor [holonManager] in
            var response = Holons_V1_MemberStatusResponse()
            if let member = await holonManager.memberStatus(slug: request.slug) {
                response.member = memberInfo(for: member)
            }
            promise.succeed(response)
        }
        return promise.futureResult
    }

    private func connectMember(
        request: Holons_V1_ConnectMemberRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_ConnectMemberResponse> {
        let promise = context.eventLoop.makePromise(of: Holons_V1_ConnectMemberResponse.self)
        Task { @MainActor [holonManager] in
            do {
                let member = try await holonManager.connectMember(
                    slug: request.slug,
                    transport: request.transport
                )
                var response = Holons_V1_ConnectMemberResponse()
                response.member = memberInfo(for: member)
                promise.succeed(response)
            } catch {
                promise.fail(grpcStatus(for: error))
            }
        }
        return promise.futureResult
    }

    private func disconnectMember(
        request: Holons_V1_DisconnectMemberRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_DisconnectMemberResponse> {
        let promise = context.eventLoop.makePromise(of: Holons_V1_DisconnectMemberResponse.self)
        Task { @MainActor [holonManager] in
            await holonManager.disconnectMember(slug: request.slug)
            promise.succeed(Holons_V1_DisconnectMemberResponse())
        }
        return promise.futureResult
    }

    private func tell(
        request: Holons_V1_TellRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_TellResponse> {
        let promise = context.eventLoop.makePromise(of: Holons_V1_TellResponse.self)
        Task { @MainActor [holonManager] in
            do {
                let payload = try await holonManager.tellMember(
                    slug: request.memberSlug,
                    method: request.method,
                    payloadJSON: Data(request.payload)
                )
                var response = Holons_V1_TellResponse()
                response.payload = payload
                promise.succeed(response)
            } catch {
                promise.fail(grpcStatus(for: error))
            }
        }
        return promise.futureResult
    }

    private func turnOffCoax(
        request: Holons_V1_TurnOffCoaxRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_TurnOffCoaxResponse> {
        _ = request
        let promise = context.eventLoop.makePromise(of: Holons_V1_TurnOffCoaxResponse.self)
        let turnOffCoax = self.turnOffCoax
        Task { @MainActor in
            turnOffCoax()
            promise.succeed(Holons_V1_TurnOffCoaxResponse())
        }
        return promise.futureResult
    }

    private func memberInfo(for member: CoaxMember) -> Holons_V1_MemberInfo {
        var info = Holons_V1_MemberInfo()
        info.slug = member.slug
        info.identity = .with {
            $0.familyName = member.familyName
            $0.givenName = member.displayName
        }
        info.state = switch member.state {
        case .available:
            .available
        case .connected:
            .connected
        case .error:
            .error
        }
        info.isOrganism = member.isOrganism
        return info
    }

    private func grpcStatus(for error: Error) -> GRPCStatus {
        if let status = error as? GRPCStatus {
            return status
        }
        return GRPCStatus(code: .unavailable, message: error.localizedDescription)
    }
}

@available(*, deprecated, renamed: "CoaxRpcServiceProvider")
public typealias CoaxServiceProvider = CoaxRpcServiceProvider
