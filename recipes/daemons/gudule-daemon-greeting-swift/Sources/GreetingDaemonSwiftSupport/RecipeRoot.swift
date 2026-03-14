import Foundation

func locateGreetingDaemonSwiftRecipeRoot() -> URL? {
    let cwd = (FileManager.default.currentDirectoryPath as NSString).standardizingPath
    let executablePath = resolveExecutablePath(from: cwd)
    let starts = [
        cwd,
        (executablePath as NSString).deletingLastPathComponent,
    ]

    for start in starts {
        var current = (start as NSString).standardizingPath
        while true {
            let holonYAML = (current as NSString).appendingPathComponent("holon.yaml")
            let packageSwift = (current as NSString).appendingPathComponent("Package.swift")
            if FileManager.default.fileExists(atPath: holonYAML),
               FileManager.default.fileExists(atPath: packageSwift) {
                return URL(fileURLWithPath: current, isDirectory: true)
            }

            let parent = (current as NSString).deletingLastPathComponent
            if parent == current || parent.isEmpty {
                break
            }
            current = parent
        }
    }

    return nil
}

func findGreetingDaemonSwiftRecipeRoot() throws -> URL {
    if let root = locateGreetingDaemonSwiftRecipeRoot() {
        return root
    }

    throw NSError(
        domain: "GreetingDaemonSwift",
        code: 1,
        userInfo: [NSLocalizedDescriptionKey: "could not locate gudule-daemon-greeting-swift recipe root"]
    )
}

private func resolveExecutablePath(from cwd: String) -> String {
    let raw = CommandLine.arguments[0]
    if (raw as NSString).isAbsolutePath {
        return (raw as NSString).standardizingPath
    }
    return ((cwd as NSString).appendingPathComponent(raw) as NSString).standardizingPath
}
