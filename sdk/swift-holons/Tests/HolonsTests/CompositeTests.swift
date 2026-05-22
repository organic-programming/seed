import XCTest

@testable import Holons

#if os(Linux)
  import Glibc
#else
  import Darwin
#endif

final class CompositeTests: XCTestCase {
  func testMemberResolvesExecutableRelativeToLauncher() throws {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("swift-composite-\(UUID().uuidString)", isDirectory: true)
    defer { try? FileManager.default.removeItem(at: root) }

    let launcher = root
      .appendingPathComponent("bin", isDirectory: true)
      .appendingPathComponent("darwin_arm64", isDirectory: true)
      .appendingPathComponent("parent")
    let memberDir = launcher
      .deletingLastPathComponent()
      .appendingPathComponent("holons", isDirectory: true)
      .appendingPathComponent("swift-node", isDirectory: true)
    let member = memberDir.appendingPathComponent("observability-cascade-swift-node")

    try FileManager.default.createDirectory(at: memberDir, withIntermediateDirectories: true)
    FileManager.default.createFile(atPath: launcher.path, contents: Data("#!/bin/sh\n".utf8))
    FileManager.default.createFile(atPath: member.path, contents: Data("#!/bin/sh\n".utf8))
    chmod(launcher.path, 0o755)
    chmod(member.path, 0o755)

    let resolved = try Composite.member(fromExecutable: launcher.path, id: "swift-node")
    XCTAssertTrue(resolved.hasSuffix("/bin/darwin_arm64/holons/swift-node/observability-cascade-swift-node"))
    XCTAssertTrue(FileManager.default.fileExists(atPath: resolved))
  }

  func testMemberErrorsWhenMissing() throws {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("swift-composite-\(UUID().uuidString)", isDirectory: true)
    defer { try? FileManager.default.removeItem(at: root) }

    let launcher = root
      .appendingPathComponent("bin", isDirectory: true)
      .appendingPathComponent("darwin_arm64", isDirectory: true)
      .appendingPathComponent("parent")
    try FileManager.default.createDirectory(at: launcher.deletingLastPathComponent(), withIntermediateDirectories: true)
    FileManager.default.createFile(atPath: launcher.path, contents: Data("#!/bin/sh\n".utf8))

    XCTAssertThrowsError(try Composite.member(fromExecutable: launcher.path, id: "swift-node"))
  }
}
