import Foundation
import Holons
import SwiftProtobuf

public enum CLI {
    public static let version = "gabriel-greeting-swift 0.1.9"

    public static func run(_ args: [String], serve: ((String, Bool) throws -> Void)? = nil) -> Int {
        var stdout = FileTextOutputStream.standardOutput
        var stderr = FileTextOutputStream.standardError
        return run(args, serve: serve, stdout: &stdout, stderr: &stderr)
    }

    public static func run<Stdout: TextOutputStream, Stderr: TextOutputStream>(
        _ args: [String],
        serve: ((String, Bool) throws -> Void)? = nil,
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
                try serve(parsed.listenURI, parsed.reflect)
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
        case "listlanguages":
            return runListLanguages(Array(args.dropFirst()), stdout: &stdout, stderr: &stderr)
        case "sayhello":
            return runSayHello(Array(args.dropFirst()), stdout: &stdout, stderr: &stderr)
        default:
            stderr.write("unknown command \"\(command)\"\n")
            printUsage(to: &stderr)
            return 1
        }
    }

    private static func runListLanguages<Stdout: TextOutputStream, Stderr: TextOutputStream>(
        _ args: [String],
        stdout: inout Stdout,
        stderr: inout Stderr
    ) -> Int {
        do {
            let (options, positional) = try parseCommandOptions(args)
            guard positional.isEmpty else {
                stderr.write("list-languages: accepts no positional arguments\n")
                return 1
            }
            return writeProto(GreetingAPI.listLanguages(), format: options.format, stdout: &stdout, stderr: &stderr, context: "list-languages")
        } catch {
            stderr.write("list-languages: \(error)\n")
            return 1
        }
    }

    private static func runSayHello<Stdout: TextOutputStream, Stderr: TextOutputStream>(
        _ args: [String],
        stdout: inout Stdout,
        stderr: inout Stderr
    ) -> Int {
        do {
            let (options, positional) = try parseCommandOptions(args)
            guard positional.count <= 2 else {
                stderr.write("say-hello: accepts at most <name> [lang_code]\n")
                return 1
            }

            var request = Greeting_V1_SayHelloRequest()
            request.langCode = "en"

            if let name = options.name {
                request.name = name
            }
            if positional.count >= 1 {
                if options.name != nil {
                    stderr.write("say-hello: use either a positional name or --name, not both\n")
                    return 1
                }
                request.name = positional[0]
            }
            if positional.count >= 2 {
                if !options.lang.isEmpty {
                    stderr.write("say-hello: use either a positional lang_code or --lang, not both\n")
                    return 1
                }
                request.langCode = positional[1]
            }
            if !options.lang.isEmpty {
                request.langCode = options.lang
            }

            return writeProto(GreetingAPI.sayHello(request), format: options.format, stdout: &stdout, stderr: &stderr, context: "say-hello")
        } catch {
            stderr.write("say-hello: \(error)\n")
            return 1
        }
    }

    private static func parseCommandOptions(_ args: [String]) throws -> (CommandOptions, [String]) {
        var options = CommandOptions()
        var positional: [String] = []
        var index = 0

        while index < args.count {
            let arg = args[index]
            switch arg {
            case "--json":
                options.format = .json
            case "--format":
                index += 1
                guard index < args.count else {
                    throw CLIError.missingValue("--format")
                }
                options.format = try OutputFormat(args[index])
            case "--lang":
                index += 1
                guard index < args.count else {
                    throw CLIError.missingValue("--lang")
                }
                options.lang = args[index].trimmingCharacters(in: .whitespacesAndNewlines)
            case "--name":
                index += 1
                guard index < args.count else {
                    throw CLIError.missingValue("--name")
                }
                options.name = args[index]
            default:
                if arg.hasPrefix("--format=") {
                    options.format = try OutputFormat(String(arg.dropFirst("--format=".count)))
                } else if arg.hasPrefix("--lang=") {
                    options.lang = String(arg.dropFirst("--lang=".count)).trimmingCharacters(in: .whitespacesAndNewlines)
                } else if arg.hasPrefix("--name=") {
                    options.name = String(arg.dropFirst("--name=".count))
                } else if arg.hasPrefix("--") {
                    throw CLIError.unknownFlag(arg)
                } else {
                    positional.append(arg)
                }
            }
            index += 1
        }

        return (options, positional)
    }

    private static func writeProto<MessageType: SwiftProtobuf.Message, Stdout: TextOutputStream, Stderr: TextOutputStream>(
        _ message: MessageType,
        format: OutputFormat,
        stdout: inout Stdout,
        stderr: inout Stderr,
        context: String
    ) -> Int {
        do {
            switch format {
            case .json:
                stdout.write(try message.jsonString())
                stdout.write("\n")
            case .text:
                try writeText(message, to: &stdout)
            }
            return 0
        } catch {
            stderr.write("\(context): \(error)\n")
            return 1
        }
    }

    private static func writeText<MessageType: SwiftProtobuf.Message, Stdout: TextOutputStream>(
        _ message: MessageType,
        to stdout: inout Stdout
    ) throws {
        switch message {
        case let typed as Greeting_V1_SayHelloResponse:
            stdout.write("\(typed.greeting)\n")
        case let typed as Greeting_V1_ListLanguagesResponse:
            for language in typed.languages {
                stdout.write("\(language.code)\t\(language.name)\t\(language.native)\n")
            }
        default:
            throw CLIError.unsupportedTextOutput(String(describing: MessageType.self))
        }
    }

    private static func canonicalCommand(_ raw: String) -> String {
        raw
            .lowercased()
            .replacingOccurrences(of: "-", with: "")
            .replacingOccurrences(of: "_", with: "")
            .replacingOccurrences(of: " ", with: "")
    }

    private static func printUsage<Output: TextOutputStream>(to output: inout Output) {
        output.write("usage: gabriel-greeting-swift <command> [args] [flags]\n")
        output.write("\n")
        output.write("commands:\n")
        output.write("  serve [--listen <uri>] [--reflect]        Start the gRPC server\n")
        output.write("  version                                  Print version and exit\n")
        output.write("  list-languages [--format text|json]      List supported languages\n")
        output.write("  say-hello [name] [lang_code] [--name <name>] [--lang <code>] [--format text|json]\n")
        output.write("  help                                     Show this help\n")
        output.write("\n")
        output.write("examples:\n")
        output.write("  gabriel-greeting-swift serve --listen stdio\n")
        output.write("  gabriel-greeting-swift list-languages --format json\n")
        output.write("  gabriel-greeting-swift say-hello Bob fr\n")
        output.write("  gabriel-greeting-swift say-hello --name Bob --lang fr --format json\n")
    }
}

private struct CommandOptions {
    var format: OutputFormat = .text
    var lang = ""
    var name: String?
}

private enum OutputFormat {
    case text
    case json

    init(_ rawValue: String) throws {
        switch rawValue.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
        case "", "text", "txt":
            self = .text
        case "json":
            self = .json
        default:
            throw CLIError.unsupportedFormat(rawValue)
        }
    }
}

private enum CLIError: Error, CustomStringConvertible {
    case missingValue(String)
    case unknownFlag(String)
    case unsupportedFormat(String)
    case unsupportedTextOutput(String)

    var description: String {
        switch self {
        case .missingValue(let flag):
            return "\(flag) requires a value"
        case .unknownFlag(let flag):
            return "unknown flag \(flag)"
        case .unsupportedFormat(let format):
            return "unsupported format \(format)"
        case .unsupportedTextOutput(let type):
            return "unsupported text output for \(type)"
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
