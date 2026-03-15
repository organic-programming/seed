import Foundation
import GRPC
import NIOCore

final class ReflectionProvider: Grpc_Reflection_V1alpha_ServerReflectionProvider {
    private static let greetingDescriptor = Data(base64Encoded: "ChF2MS9ncmVldGluZy5wcm90bxILZ3JlZXRpbmcudjEiFgoUTGlzdExhbmd1YWdlc1JlcXVlc3QiTAoVTGlzdExhbmd1YWdlc1Jlc3BvbnNlEjMKCWxhbmd1YWdlcxgBIAMoCzIVLmdyZWV0aW5nLnYxLkxhbmd1YWdlUglsYW5ndWFnZXMiSgoITGFuZ3VhZ2USEgoEY29kZRgBIAEoCVIEY29kZRISCgRuYW1lGAIgASgJUgRuYW1lEhYKBm5hdGl2ZRgDIAEoCVIGbmF0aXZlIkIKD1NheUhlbGxvUmVxdWVzdBISCgRuYW1lGAEgASgJUgRuYW1lEhsKCWxhbmdfY29kZRgCIAEoCVIIbGFuZ0NvZGUiZwoQU2F5SGVsbG9SZXNwb25zZRIaCghncmVldGluZxgBIAEoCVIIZ3JlZXRpbmcSGgoIbGFuZ3VhZ2UYAiABKAlSCGxhbmd1YWdlEhsKCWxhbmdfY29kZRgDIAEoCVIIbGFuZ0NvZGUysgEKD0dyZWV0aW5nU2VydmljZRJWCg1MaXN0TGFuZ3VhZ2VzEiEuZ3JlZXRpbmcudjEuTGlzdExhbmd1YWdlc1JlcXVlc3QaIi5ncmVldGluZy52MS5MaXN0TGFuZ3VhZ2VzUmVzcG9uc2USRwoIU2F5SGVsbG8SHC5ncmVldGluZy52MS5TYXlIZWxsb1JlcXVlc3QaHS5ncmVldGluZy52MS5TYXlIZWxsb1Jlc3BvbnNlYgZwcm90bzM=")!
    private static let reflectionDescriptor = Data(base64Encoded: "CihncnBjL3JlZmxlY3Rpb24vdjFhbHBoYS9yZWZsZWN0aW9uLnByb3RvEhdncnBjLnJlZmxlY3Rpb24udjFhbHBoYSL4AgoXU2VydmVyUmVmbGVjdGlvblJlcXVlc3QSEgoEaG9zdBgBIAEoCVIEaG9zdBIqChBmaWxlX2J5X2ZpbGVuYW1lGAMgASgJSABSDmZpbGVCeUZpbGVuYW1lEjYKFmZpbGVfY29udGFpbmluZ19zeW1ib2wYBCABKAlIAFIUZmlsZUNvbnRhaW5pbmdTeW1ib2wSZwoZZmlsZV9jb250YWluaW5nX2V4dGVuc2lvbhgFIAEoCzIpLmdycGMucmVmbGVjdGlvbi52MWFscGhhLkV4dGVuc2lvblJlcXVlc3RIAFIXZmlsZUNvbnRhaW5pbmdFeHRlbnNpb24SQgodYWxsX2V4dGVuc2lvbl9udW1iZXJzX29mX3R5cGUYBiABKAlIAFIZYWxsRXh0ZW5zaW9uTnVtYmVyc09mVHlwZRIlCg1saXN0X3NlcnZpY2VzGAcgASgJSABSDGxpc3RTZXJ2aWNlc0IRCg9tZXNzYWdlX3JlcXVlc3QiZgoQRXh0ZW5zaW9uUmVxdWVzdBInCg9jb250YWluaW5nX3R5cGUYASABKAlSDmNvbnRhaW5pbmdUeXBlEikKEGV4dGVuc2lvbl9udW1iZXIYAiABKAVSD2V4dGVuc2lvbk51bWJlciLHBAoYU2VydmVyUmVmbGVjdGlvblJlc3BvbnNlEh0KCnZhbGlkX2hvc3QYASABKAlSCXZhbGlkSG9zdBJbChBvcmlnaW5hbF9yZXF1ZXN0GAIgASgLMjAuZ3JwYy5yZWZsZWN0aW9uLnYxYWxwaGEuU2VydmVyUmVmbGVjdGlvblJlcXVlc3RSD29yaWdpbmFsUmVxdWVzdBJrChhmaWxlX2Rlc2NyaXB0b3JfcmVzcG9uc2UYBCABKAsyLy5ncnBjLnJlZmxlY3Rpb24udjFhbHBoYS5GaWxlRGVzY3JpcHRvclJlc3BvbnNlSABSFmZpbGVEZXNjcmlwdG9yUmVzcG9uc2USdwoeYWxsX2V4dGVuc2lvbl9udW1iZXJzX3Jlc3BvbnNlGAUgASgLMjAuZ3JwYy5yZWZsZWN0aW9uLnYxYWxwaGEuRXh0ZW5zaW9uTnVtYmVyUmVzcG9uc2VIAFIbYWxsRXh0ZW5zaW9uTnVtYmVyc1Jlc3BvbnNlEmQKFmxpc3Rfc2VydmljZXNfcmVzcG9uc2UYBiABKAsyLC5ncnBjLnJlZmxlY3Rpb24udjFhbHBoYS5MaXN0U2VydmljZVJlc3BvbnNlSABSFGxpc3RTZXJ2aWNlc1Jlc3BvbnNlEk8KDmVycm9yX3Jlc3BvbnNlGAcgASgLMiYuZ3JwYy5yZWZsZWN0aW9uLnYxYWxwaGEuRXJyb3JSZXNwb25zZUgAUg1lcnJvclJlc3BvbnNlQhIKEG1lc3NhZ2VfcmVzcG9uc2UiTAoWRmlsZURlc2NyaXB0b3JSZXNwb25zZRIyChVmaWxlX2Rlc2NyaXB0b3JfcHJvdG8YASADKAxSE2ZpbGVEZXNjcmlwdG9yUHJvdG8iagoXRXh0ZW5zaW9uTnVtYmVyUmVzcG9uc2USJAoOYmFzZV90eXBlX25hbWUYASABKAlSDGJhc2VUeXBlTmFtZRIpChBleHRlbnNpb25fbnVtYmVyGAIgAygFUg9leHRlbnNpb25OdW1iZXIiWQoTTGlzdFNlcnZpY2VSZXNwb25zZRJCCgdzZXJ2aWNlGAEgAygLMiguZ3JwYy5yZWZsZWN0aW9uLnYxYWxwaGEuU2VydmljZVJlc3BvbnNlUgdzZXJ2aWNlIiUKD1NlcnZpY2VSZXNwb25zZRISCgRuYW1lGAEgASgJUgRuYW1lIlMKDUVycm9yUmVzcG9uc2USHQoKZXJyb3JfY29kZRgBIAEoBVIJZXJyb3JDb2RlEiMKDWVycm9yX21lc3NhZ2UYAiABKAlSDGVycm9yTWVzc2FnZTKTAQoQU2VydmVyUmVmbGVjdGlvbhJ/ChRTZXJ2ZXJSZWZsZWN0aW9uSW5mbxIwLmdycGMucmVmbGVjdGlvbi52MWFscGhhLlNlcnZlclJlZmxlY3Rpb25SZXF1ZXN0GjEuZ3JwYy5yZWZsZWN0aW9uLnYxYWxwaGEuU2VydmVyUmVmbGVjdGlvblJlc3BvbnNlKAEwAWIGcHJvdG8z")!

    func serverReflectionInfo(
        context: StreamingResponseCallContext<Grpc_Reflection_V1alpha_ServerReflectionResponse>
    ) -> EventLoopFuture<(StreamEvent<Grpc_Reflection_V1alpha_ServerReflectionRequest>) -> Void> {
        context.eventLoop.makeSucceededFuture { event in
            switch event {
            case .message(let request):
                let response = Self.makeResponse(for: request)
                context.sendResponse(response, promise: nil)
            case .end:
                context.statusPromise.succeed(.ok)
            }
        }
    }

    private static func makeResponse(
        for request: Grpc_Reflection_V1alpha_ServerReflectionRequest
    ) -> Grpc_Reflection_V1alpha_ServerReflectionResponse {
        switch request.messageRequest {
        case .listServices:
            var list = Grpc_Reflection_V1alpha_ListServiceResponse()
            list.service = [
                serviceResponse(named: "greeting.v1.GreetingService"),
                serviceResponse(named: "grpc.reflection.v1alpha.ServerReflection"),
            ]
            var response = baseResponse(for: request)
            response.listServicesResponse = list
            return response

        case .fileContainingSymbol(let symbol):
            switch symbol {
            case "greeting.v1.GreetingService":
                return descriptorResponse(for: request, descriptors: [greetingDescriptor])
            case "grpc.reflection.v1alpha.ServerReflection":
                return descriptorResponse(for: request, descriptors: [reflectionDescriptor])
            default:
                return errorResponse(for: request, code: 5, message: "symbol not found: \(symbol)")
            }

        case .fileByFilename(let filename):
            switch filename {
            case "v1/greeting.proto", "protos/v1/greeting.proto":
                return descriptorResponse(for: request, descriptors: [greetingDescriptor])
            case "grpc/reflection/v1alpha/reflection.proto":
                return descriptorResponse(for: request, descriptors: [reflectionDescriptor])
            default:
                return errorResponse(for: request, code: 5, message: "file not found: \(filename)")
            }

        case .fileContainingExtension:
            return errorResponse(for: request, code: 12, message: "file_containing_extension is not implemented")

        case .allExtensionNumbersOfType:
            return errorResponse(for: request, code: 12, message: "all_extension_numbers_of_type is not implemented")

        case .none:
            return errorResponse(for: request, code: 3, message: "empty reflection request")
        }
    }

    private static func baseResponse(
        for request: Grpc_Reflection_V1alpha_ServerReflectionRequest
    ) -> Grpc_Reflection_V1alpha_ServerReflectionResponse {
        var response = Grpc_Reflection_V1alpha_ServerReflectionResponse()
        response.validHost = request.host
        response.originalRequest = request
        return response
    }

    private static func descriptorResponse(
        for request: Grpc_Reflection_V1alpha_ServerReflectionRequest,
        descriptors: [Data]
    ) -> Grpc_Reflection_V1alpha_ServerReflectionResponse {
        var payload = Grpc_Reflection_V1alpha_FileDescriptorResponse()
        payload.fileDescriptorProto = descriptors

        var response = baseResponse(for: request)
        response.fileDescriptorResponse = payload
        return response
    }

    private static func errorResponse(
        for request: Grpc_Reflection_V1alpha_ServerReflectionRequest,
        code: Int32,
        message: String
    ) -> Grpc_Reflection_V1alpha_ServerReflectionResponse {
        var payload = Grpc_Reflection_V1alpha_ErrorResponse()
        payload.errorCode = code
        payload.errorMessage = message

        var response = baseResponse(for: request)
        response.errorResponse = payload
        return response
    }

    private static func serviceResponse(named name: String) -> Grpc_Reflection_V1alpha_ServiceResponse {
        var response = Grpc_Reflection_V1alpha_ServiceResponse()
        response.name = name
        return response
    }
}
