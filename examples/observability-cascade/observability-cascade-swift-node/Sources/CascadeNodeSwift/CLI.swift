import Foundation
import Holons
import SwiftProtobuf

public enum CLI {
    public static let version = "observability-cascade-swift-node {{ .Version }}"

    public static func run(_ args: [String], serve: ((String, Bool, [Serve.MemberRef]) throws -> Void)? = nil) -> Int {
        var stdout = FileTextOutputStream.standardOutput
        var stderr = FileTextOutputStream.standardError
        return run(args, serve: serve, stdout: &stdout, stderr: &stderr)
    }

    public static func run<Stdout: TextOutputStream, Stderr: TextOutputStream>(
        _ args: [String],
        serve: ((String, Bool, [Serve.MemberRef]) throws -> Void)? = nil,
        stdout: inout Stdout,
        stderr: inout Stderr
    ) -> Int {
        guard let command = args.first else {
            printUsage(to: &stderr)
            return 1
        }

        switch canonicalCommand(command) {
        case "serve":
            let parsed = Serve.parseOptions(Array(args.dropFirst()))
            guard let serve else {
                stderr.write("serve: not available\n")
                return 1
            }
            do {
                let members = try validateMembers(parsed.memberEndpoints)
                try serve(parsed.listenURI, parsed.reflect, members)
                return 0
            } catch {
                stderr.write("serve: \(error)\n")
                return 1
            }
        case "version":
            stdout.write("\(version)\n")
            return 0
        case "help":
            printUsage(to: &stdout)
            return 0
        case "tick":
            return runTick(Array(args.dropFirst()), stdout: &stdout, stderr: &stderr)
        default:
            stderr.write("unknown command \"\(command)\"\n")
            printUsage(to: &stderr)
            return 1
        }
    }

    private static func runTick<Stdout: TextOutputStream, Stderr: TextOutputStream>(
        _ args: [String],
        stdout: inout Stdout,
        stderr: inout Stderr
    ) -> Int {
        do {
            var request = Relay_V1_TickRequest()
            var positional: [String] = []
            var index = 0
            while index < args.count {
                let arg = args[index]
                switch arg {
                case "--sender":
                    index += 1
                    guard index < args.count else { throw CLIError.missingValue("--sender") }
                    request.sender = args[index]
                case "--note":
                    index += 1
                    guard index < args.count else { throw CLIError.missingValue("--note") }
                    request.note = args[index]
                default:
                    if arg.hasPrefix("--sender=") {
                        request.sender = String(arg.dropFirst("--sender=".count))
                    } else if arg.hasPrefix("--note=") {
                        request.note = String(arg.dropFirst("--note=".count))
                    } else if arg.hasPrefix("--") {
                        throw CLIError.unknownFlag(arg)
                    } else {
                        positional.append(arg)
                    }
                }
                index += 1
            }

            if request.sender.isEmpty, let first = positional.first {
                request.sender = first
            }
            if request.note.isEmpty, positional.count >= 2 {
                request.note = positional[1]
            }

            try writeJSON(PublicAPI.tick(request), stdout: &stdout)
            return 0
        } catch {
            stderr.write("tick: \(error)\n")
            return 1
        }
    }

    private static func validateMembers(_ members: [Serve.MemberRef]) throws -> [Serve.MemberRef] {
        try members.map { member in
            let slug = member.slug.trimmingCharacters(in: .whitespacesAndNewlines)
            let address = member.address.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !slug.isEmpty, !address.isEmpty else {
                throw CLIError.invalidMember
            }
            return Serve.MemberRef(slug: slug, address: address)
        }
    }

    private static func writeJSON<MessageType: SwiftProtobuf.Message, Output: TextOutputStream>(
        _ message: MessageType,
        stdout: inout Output
    ) throws {
        stdout.write(try message.jsonString())
        stdout.write("\n")
    }

    private static func canonicalCommand(_ raw: String) -> String {
        raw
            .lowercased()
            .replacingOccurrences(of: "-", with: "")
            .replacingOccurrences(of: "_", with: "")
            .replacingOccurrences(of: " ", with: "")
    }

    private static func printUsage<Output: TextOutputStream>(to output: inout Output) {
        output.write("usage: observability-cascade-swift-node <command> [args] [flags]\n")
        output.write("\n")
        output.write("commands:\n")
        output.write("  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server\n")
        output.write("  tick [sender] [note]                                Emit one local tick\n")
        output.write("  version                                             Print version and exit\n")
        output.write("  help                                                Print usage\n")
    }
}

private enum CLIError: Error, CustomStringConvertible {
    case missingValue(String)
    case unknownFlag(String)
    case invalidMember

    var description: String {
        switch self {
        case .missingValue(let flag):
            return "\(flag) requires a value"
        case .unknownFlag(let flag):
            return "unknown flag \(flag)"
        case .invalidMember:
            return "--member requires <slug>=<address>"
        }
    }
}

public struct FileTextOutputStream: TextOutputStream {
    public static let standardOutput = FileTextOutputStream(fileHandle: .standardOutput)
    public static let standardError = FileTextOutputStream(fileHandle: .standardError)

    private let fileHandle: FileHandle

    public init(fileHandle: FileHandle) {
        self.fileHandle = fileHandle
    }

    public mutating func write(_ string: String) {
        guard let data = string.data(using: .utf8) else {
            return
        }
        try? fileHandle.write(contentsOf: data)
    }
}
