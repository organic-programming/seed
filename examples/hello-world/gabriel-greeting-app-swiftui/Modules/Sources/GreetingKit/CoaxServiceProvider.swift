import Foundation
import GRPC
import NIOCore
import SwiftProtobuf

// CoaxServiceProvider implements the COAX interaction surface (holons.v1.CoaxService)
// for the Gabriel Greeting App organism. It drives the same HolonProcess state
// that the SwiftUI views observe — agent actions appear in the UI in real time.
final class CoaxServiceProvider: CallHandlerProvider, @unchecked Sendable {
    let serviceName: Substring = "holons.v1.CoaxService"

    private let holon: HolonProcess
    private weak var coaxServer: CoaxServer?

    init(holon: HolonProcess, coaxServer: CoaxServer) {
        self.holon = holon
        self.coaxServer = coaxServer
    }

    func handle(method name: Substring, context: CallHandlerContext) -> GRPCServerHandlerProtocol? {
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
                requestDeserializer: ProtobufDeserializer<Holons_V1_ListMembersRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_DisconnectMemberResponse>(),
                interceptors: [],
                userFunction: turnOffCoax(request:context:)
            )
        default:
            return nil
        }
    }

    // MARK: - RPC Implementations

    private func listMembers(
        request: Holons_V1_ListMembersRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_ListMembersResponse> {
        let promise = context.eventLoop.makePromise(of: Holons_V1_ListMembersResponse.self)
        Task { @MainActor [holon] in
            var response = Holons_V1_ListMembersResponse()
            response.members = holon.availableHolons.map { identity in
                var member = Holons_V1_MemberInfo()
                member.slug = identity.slug
                var mid = Holons_V1_HolonManifest.Identity()
                mid.familyName = identity.familyName
                mid.givenName = identity.displayName
                member.identity = mid
                if holon.selectedHolon?.slug == identity.slug && holon.isRunning {
                    member.state = .connected
                } else {
                    member.state = .available
                }
                return member
            }
            promise.succeed(response)
        }
        return promise.futureResult
    }

    private func memberStatus(
        request: Holons_V1_MemberStatusRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_MemberStatusResponse> {
        let promise = context.eventLoop.makePromise(of: Holons_V1_MemberStatusResponse.self)
        Task { @MainActor [holon] in
            var response = Holons_V1_MemberStatusResponse()
            if let identity = holon.availableHolons.first(where: { $0.slug == request.slug }) {
                var member = Holons_V1_MemberInfo()
                member.slug = identity.slug
                if holon.selectedHolon?.slug == identity.slug && holon.isRunning {
                    member.state = .connected
                } else {
                    member.state = .available
                }
                response.member = member
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
        Task { @MainActor [holon] in
            guard let identity = holon.availableHolons.first(where: { $0.slug == request.slug }) else {
                promise.fail(GRPCStatus(code: .notFound, message: "Member '\(request.slug)' not found"))
                return
            }
            if !request.transport.isEmpty {
                holon.transport = request.transport
            }
            holon.selectedHolon = identity
            await holon.start()
            var response = Holons_V1_ConnectMemberResponse()
            var member = Holons_V1_MemberInfo()
            member.slug = identity.slug
            member.state = holon.isRunning ? .connected : .error
            response.member = member
            promise.succeed(response)
        }
        return promise.futureResult
    }

    private func disconnectMember(
        request: Holons_V1_DisconnectMemberRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_DisconnectMemberResponse> {
        let promise = context.eventLoop.makePromise(of: Holons_V1_DisconnectMemberResponse.self)
        Task { @MainActor [holon] in
            holon.stop()
            promise.succeed(Holons_V1_DisconnectMemberResponse())
        }
        return promise.futureResult
    }

    private func tell(
        request: Holons_V1_TellRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_TellResponse> {
        // Tell is not implemented in this initial wiring — domain verbs
        // (GreetingAppService) cover the primary use cases.
        return context.eventLoop.makeFailedFuture(
            GRPCStatus(code: .unimplemented, message: "Tell is not yet implemented")
        )
    }

    private func turnOffCoax(
        request: Holons_V1_ListMembersRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_DisconnectMemberResponse> {
        _ = request
        let promise = context.eventLoop.makePromise(of: Holons_V1_DisconnectMemberResponse.self)
        let eventLoop = context.eventLoop
        Task { @MainActor [weak coaxServer] in
            eventLoop.execute {
                promise.succeed(Holons_V1_DisconnectMemberResponse())
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + .milliseconds(100)) {
                coaxServer?.isEnabled = false
            }
        }
        return promise.futureResult
    }
}
