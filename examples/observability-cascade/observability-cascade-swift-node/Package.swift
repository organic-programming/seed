// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "observability-cascade-swift-node",
    platforms: [.macOS(.v13)],
    products: [
        .library(name: "CascadeNodeSwift", targets: ["CascadeNodeSwift"]),
        .library(name: "CascadeNodeSwiftServer", targets: ["CascadeNodeSwiftServer"]),
        .executable(name: "observability-cascade-swift-node", targets: ["observability-cascade-swift-node"]),
    ],
    dependencies: [
        .package(path: "../../../sdk/swift-holons"),
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
    ],
    targets: [
        .target(
            name: "CascadeNodeSwift",
            dependencies: [
                .product(name: "Holons", package: "swift-holons"),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIO", package: "swift-nio"),
                .product(name: "NIOConcurrencyHelpers", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: ".",
            exclude: [
                "Sources/CascadeNodeSwiftServer",
                "Sources/observability-cascade-swift-node",
                "api",
            ],
            sources: [
                "Sources/CascadeNodeSwift",
                "gen/describe_generated.swift",
                "gen/swift/relay/v1",
            ]
        ),
        .target(
            name: "CascadeNodeSwiftServer",
            dependencies: [
                "CascadeNodeSwift",
                .product(name: "Holons", package: "swift-holons"),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIO", package: "swift-nio"),
            ],
            path: "Sources/CascadeNodeSwiftServer"
        ),
        .executableTarget(
            name: "observability-cascade-swift-node",
            dependencies: [
                "CascadeNodeSwift",
                "CascadeNodeSwiftServer",
            ],
            path: "Sources/observability-cascade-swift-node"
        ),
    ]
)
