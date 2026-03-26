import Foundation
import GRPC
import NIOCore
import NIOPosix
import SwiftProtobuf
import XCTest

@testable import Holons

final class ProtoLessDescribeTests: XCTestCase {
  func testBuiltBinaryServesStaticDescribeWithoutAdjacentProtoFiles() throws {
    let sourceRoot = try writeManifestOnlyHolon()
    defer { try? FileManager.default.removeItem(at: sourceRoot) }

    let response = try Describe.buildResponse(protoDir: sourceRoot.path)
    let payload = try response.serializedData().base64EncodedString()
    let built = try prepareProtoLessServerRun()
    defer { try? FileManager.default.removeItem(at: built.root) }

    let process = Process()
    process.executableURL = built.executable
    process.arguments = ["tcp://127.0.0.1:0"]
    process.currentDirectoryURL = built.runDirectory
    process.environment = ProcessInfo.processInfo.environment.merging(
      ["PROTOLESS_STATIC_DESCRIBE_BASE64": payload],
      uniquingKeysWith: { _, new in new }
    )

    let stdout = Pipe()
    let stderr = Pipe()
    process.standardOutput = stdout
    process.standardError = stderr

    try process.run()
    defer { stopProcess(process) }
    let stdoutLines = ProtoLessLineQueue()
    let stderrLines = ProtoLessStringCollector()
    startLineReader(handle: stdout.fileHandleForReading, queue: stdoutLines, collector: nil)
    startLineReader(handle: stderr.fileHandleForReading, queue: nil, collector: stderrLines)

    let advertisedURI = try waitForFirstLine(
      process: process,
      stdout: stdoutLines,
      stderr: stderrLines
    )
    XCTAssertFalse(
      FileManager.default.fileExists(
        atPath: built.runDirectory.appendingPathComponent(Identity.protoManifestFileName).path
      )
    )

    let parsed = try Transport.parse(advertisedURI)
    let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)
    let channel = ClientConnection.insecure(group: group)
      .connect(host: parsed.host ?? "127.0.0.1", port: parsed.port ?? 0)

    defer {
      _ = try? channel.close().wait()
      try? group.syncShutdownGracefully()
    }

    let call =
      channel.makeUnaryCall(
        path: "/holons.v1.HolonMeta/Describe",
        request: ProtoLessPayload(message: Holons_V1_DescribeRequest()),
        callOptions: CallOptions()
      )
      as UnaryCall<
        ProtoLessPayload<Holons_V1_DescribeRequest>,
        ProtoLessPayload<Holons_V1_DescribeResponse>
      >
    let served = try call.response.wait().message

    XCTAssertEqual(served.manifest.identity.givenName, "Proto")
    XCTAssertEqual(served.manifest.identity.familyName, "Less")
    XCTAssertEqual(served.manifest.identity.motto, "Serve static describe only.")
    XCTAssertTrue(served.services.isEmpty)
  }
}

private struct BuiltProtoLessServer {
  let root: URL
  let executable: URL
  let runDirectory: URL
}

private struct ProtoLessPayload<MessageType: SwiftProtobuf.Message & Sendable>: GRPCPayload, Sendable {
  let message: MessageType

  init(message: MessageType) {
    self.message = message
  }

  init(serializedByteBuffer: inout ByteBuffer) throws {
    let data = serializedByteBuffer.readData(length: serializedByteBuffer.readableBytes) ?? Data()
    self.message = try MessageType(serializedBytes: data)
  }

  func serialize(into buffer: inout ByteBuffer) throws {
    let bytes: [UInt8] = try message.serializedBytes()
    buffer.writeBytes(bytes)
  }
}

private func writeManifestOnlyHolon() throws -> URL {
  let root = FileManager.default.temporaryDirectory
    .appendingPathComponent("swift_holons_protoless_source_\(UUID().uuidString)", isDirectory: true)
  try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
  try writeHolonProto(
    to: root.appendingPathComponent("holon.proto"),
    packageName: "protoless.v1",
    uuid: "proto-less-uuid",
    givenName: "Proto",
    familyName: "Less",
    motto: "Serve static describe only.",
    lang: "swift",
    kind: "native",
    buildRunner: "swift-package",
    artifactBinary: "proto-less-server"
  )
  return root
}

private func prepareProtoLessServerRun() throws -> BuiltProtoLessServer {
  let root = FileManager.default.temporaryDirectory
    .appendingPathComponent("swift_holons_protoless_run_\(UUID().uuidString)", isDirectory: true)
  let runDirectory = root.appendingPathComponent("run", isDirectory: true)
  try FileManager.default.createDirectory(at: runDirectory, withIntermediateDirectories: true)

  guard
    let executable = findBuiltExecutable(
      named: "protoless-describe-fixture",
      under: CertificationCLI.packageRoot().appendingPathComponent(".build")
    )
  else {
    throw NSError(
      domain: "Holons.ProtoLessDescribeTests",
      code: 1,
      userInfo: [NSLocalizedDescriptionKey: "failed to locate built protoless-describe-fixture executable"]
    )
  }

  let copiedExecutable = runDirectory.appendingPathComponent("protoless-describe-fixture")
  try FileManager.default.copyItem(at: executable, to: copiedExecutable)
  return BuiltProtoLessServer(root: root, executable: copiedExecutable, runDirectory: runDirectory)
}

private func findBuiltExecutable(named name: String, under root: URL) -> URL? {
  guard
    let enumerator = FileManager.default.enumerator(
      at: root,
      includingPropertiesForKeys: [.isRegularFileKey],
      options: [],
      errorHandler: { _, _ in true }
    )
  else {
    return nil
  }

  while let candidate = enumerator.nextObject() as? URL {
    if candidate.lastPathComponent == name, FileManager.default.isExecutableFile(atPath: candidate.path) {
      return candidate
    }
  }
  return nil
}

private func waitForFirstLine(
  process: Process,
  stdout: ProtoLessLineQueue,
  stderr: ProtoLessStringCollector,
  timeout: TimeInterval = 10.0
) throws -> String {
  let deadline = Date().addingTimeInterval(timeout)

  while Date() < deadline {
    if let line = stdout.pop(timeout: 0.1) {
      return line
    }
    if !process.isRunning {
      let stderrText = stderr.text.trimmingCharacters(in: .whitespacesAndNewlines)
      throw NSError(
        domain: "Holons.ProtoLessDescribeTests",
        code: 2,
        userInfo: [
          NSLocalizedDescriptionKey: stderrText.isEmpty
            ? "server exited before advertising an address"
            : "server exited before advertising an address: \(stderrText)"
        ]
      )
    }
  }

  let stderrText = stderr.text.trimmingCharacters(in: .whitespacesAndNewlines)
  throw NSError(
    domain: "Holons.ProtoLessDescribeTests",
    code: 3,
    userInfo: [
      NSLocalizedDescriptionKey: stderrText.isEmpty
        ? "timed out waiting for server address"
        : "timed out waiting for server address: \(stderrText)"
    ]
  )
}

private func stopProcess(_ process: Process) {
  guard process.isRunning else {
    return
  }
  process.terminate()
  process.waitUntilExit()
}

private final class ProtoLessLineQueue {
  private let lock = NSLock()
  private let semaphore = DispatchSemaphore(value: 0)
  private var lines: [String] = []

  func push(_ line: String) {
    lock.lock()
    lines.append(line)
    lock.unlock()
    semaphore.signal()
  }

  func pop(timeout: TimeInterval) -> String? {
    guard semaphore.wait(timeout: .now() + timeout) == .success else {
      return nil
    }
    lock.lock()
    defer { lock.unlock() }
    guard !lines.isEmpty else {
      return nil
    }
    return lines.removeFirst()
  }
}

private final class ProtoLessStringCollector {
  private let lock = NSLock()
  private var lines: [String] = []

  func append(_ line: String) {
    lock.lock()
    lines.append(line)
    lock.unlock()
  }

  var text: String {
    lock.lock()
    defer { lock.unlock() }
    return lines.joined(separator: "\n")
  }
}

private func startLineReader(
  handle: FileHandle,
  queue: ProtoLessLineQueue?,
  collector: ProtoLessStringCollector?
) {
  DispatchQueue.global(qos: .utility).async {
    var buffer = Data()

    while true {
      let chunk = handle.availableData
      if chunk.isEmpty {
        if !buffer.isEmpty, let line = String(data: buffer, encoding: .utf8) {
          collector?.append(line)
          queue?.push(line)
        }
        return
      }

      buffer.append(chunk)
      while let newline = buffer.firstIndex(of: 0x0A) {
        let lineData = buffer.prefix(upTo: newline)
        buffer.removeSubrange(...newline)
        guard let line = String(data: lineData, encoding: .utf8) else {
          continue
        }
        collector?.append(line)
        queue?.push(line)
      }
    }
  }
}
