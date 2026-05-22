import Foundation
import Holons

public enum CLI {
    public static let version = "observability-cascade-swift-node {{ .Version }}"

    public static func run(_ args: [String], serve: ((String, String, [ChildSpec]) throws -> Void)? = nil) -> Int {
        var stdout = FileTextOutputStream.standardOutput
        var stderr = FileTextOutputStream.standardError
        return run(args, serve: serve, stdout: &stdout, stderr: &stderr)
    }

    public static func run<Stdout: TextOutputStream, Stderr: TextOutputStream>(
        _ args: [String],
        serve: ((String, String, [ChildSpec]) throws -> Void)? = nil,
        stdout: inout Stdout,
        stderr: inout Stderr
    ) -> Int {
        guard let command = args.first else {
            printUsage(to: &stderr)
            return 1
        }

        switch canonicalCommand(command) {
        case "serve":
            let parsedChildren = Serve.parseChildFlags(Array(args.dropFirst()))
            let parsed = Serve.parseOptions(parsedChildren.remaining)
            let transport = parseTransport(parsedChildren.remaining)
            guard let serve else {
                stderr.write("serve: not available\n")
                return 1
            }
            do {
                try serve(parsed.listenURI, transport, parsedChildren.children)
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
        default:
            stderr.write("unknown command \"\(command)\"\n")
            printUsage(to: &stderr)
            return 1
        }
    }

    private static func parseTransport(_ args: [String]) -> String {
        for index in args.indices {
            if args[index] == "--transport", index + 1 < args.count {
                return args[index + 1]
            }
            if args[index].hasPrefix("--transport=") {
                return String(args[index].dropFirst("--transport=".count))
            }
        }
        return "stdio"
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
        output.write("  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server\n")
        output.write("  version                                                           Print version and exit\n")
        output.write("  help                                                              Print usage\n")
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
