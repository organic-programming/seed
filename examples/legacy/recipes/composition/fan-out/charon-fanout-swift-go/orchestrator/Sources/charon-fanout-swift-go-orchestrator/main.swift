import Foundation

func resolveScript() -> String {
    if let override = ProcessInfo.processInfo.environment["CHARON_RUN_SCRIPT"], !override.isEmpty {
        return override
    }
    return URL(fileURLWithPath: CommandLine.arguments[0])
        .deletingLastPathComponent()
        .appendingPathComponent("scripts")
        .appendingPathComponent("run.sh")
        .path
}

let process = Process()
process.executableURL = URL(fileURLWithPath: "/bin/sh")
process.arguments = [resolveScript()]
process.standardInput = FileHandle.standardInput
process.standardOutput = FileHandle.standardOutput
process.standardError = FileHandle.standardError

do {
    try process.run()
    process.waitUntilExit()
    exit(process.terminationStatus)
} catch {
    FileHandle.standardError.write(Data(("\(error)\n").utf8))
    exit(1)
}
