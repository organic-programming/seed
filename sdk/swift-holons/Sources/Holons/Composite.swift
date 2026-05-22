import Foundation

#if os(Linux)
  import Glibc
#else
  import Darwin
#endif

public enum Composite {
  public static func member(_ id: String) throws -> String {
    let envPath = ProcessInfo.processInfo.environment["OP_HOLON_EXECUTABLE"]?
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let executable = (envPath?.isEmpty == false) ? envPath! : try currentExecutablePath()
    return try member(fromExecutable: executable, id: id)
  }

  public static func member(fromExecutable executable: String, id: String) throws -> String {
    guard !id.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
      throw CompositeError.invalidMemberID
    }
    guard !executable.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
      throw CompositeError.missingExecutable
    }

    let executableURL = URL(fileURLWithPath: executable).standardizedFileURL
    let memberURL = executableURL
      .deletingLastPathComponent()
      .appendingPathComponent("holons", isDirectory: true)
      .appendingPathComponent(id, isDirectory: true)
    guard FileManager.default.fileExists(atPath: memberURL.path) else {
      throw CompositeError.memberDirectoryNotFound(memberURL.path)
    }

    let entries = try FileManager.default.contentsOfDirectory(
      at: memberURL,
      includingPropertiesForKeys: nil,
      options: [.skipsHiddenFiles]
    )
    for entry in entries.sorted(by: { $0.lastPathComponent < $1.lastPathComponent }) {
      if isExecutable(entry.path) {
        return entry.path
      }
    }
    throw CompositeError.noExecutableFound(memberURL.path)
  }

  private static func currentExecutablePath() throws -> String {
    #if os(Linux)
      var buffer = [CChar](repeating: 0, count: 4096)
      let count = readlink("/proc/self/exe", &buffer, buffer.count - 1)
      guard count > 0 else {
        throw CompositeError.missingExecutable
      }
      buffer[Int(count)] = 0
      return String(cString: buffer)
    #else
      var size: UInt32 = 0
      _ = _NSGetExecutablePath(nil, &size)
      var buffer = [CChar](repeating: 0, count: Int(size))
      guard _NSGetExecutablePath(&buffer, &size) == 0 else {
        throw CompositeError.missingExecutable
      }
      return String(cString: buffer)
    #endif
  }

  private static func isExecutable(_ path: String) -> Bool {
    #if os(Windows)
      return URL(fileURLWithPath: path).pathExtension.lowercased() == "exe"
    #else
      return URL(fileURLWithPath: path).pathExtension.isEmpty &&
        FileManager.default.isExecutableFile(atPath: path)
    #endif
  }
}

public enum CompositeError: Error, CustomStringConvertible {
  case invalidMemberID
  case missingExecutable
  case memberDirectoryNotFound(String)
  case noExecutableFound(String)

  public var description: String {
    switch self {
    case .invalidMemberID:
      return "member id is required"
    case .missingExecutable:
      return "OP_HOLON_EXECUTABLE is not set"
    case .memberDirectoryNotFound(let path):
      return "member directory not found: \(path)"
    case .noExecutableFound(let path):
      return "no executable found in \(path)"
    }
  }
}
