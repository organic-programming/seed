import Foundation
import Holons
#if os(Linux)
import Glibc
#else
import Darwin
#endif

enum ProtoLessFixtureError: Error, CustomStringConvertible {
  case missingPayload

  var description: String {
    switch self {
    case .missingPayload:
      return "PROTOLESS_STATIC_DESCRIBE_BASE64 is required"
    }
  }
}

@main
struct ProtoLessDescribeFixture {
  static func main() throws {
    let payload = ProcessInfo.processInfo.environment["PROTOLESS_STATIC_DESCRIBE_BASE64"]?
      .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    guard !payload.isEmpty else {
      throw ProtoLessFixtureError.missingPayload
    }

    let listenURI = CommandLine.arguments.dropFirst().first ?? "tcp://127.0.0.1:0"
    try Describe.useStaticResponse(StaticDescribeResponse(payloadBase64: payload))
    try Serve.runWithOptions(
      listenURI,
      serviceProviders: [],
      options: Serve.Options(
        onListen: { uri in
          print(uri)
          fflush(stdout)
        }
      )
    )
  }
}
