// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "observability-cascade-swift",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "observability-cascade-swift", targets: ["observability-cascade-swift"]),
    ],
    dependencies: [
        .package(path: "../../../sdk/swift-holons"),
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
    ],
    targets: [
        .executableTarget(
            name: "observability-cascade-swift",
            dependencies: [
                .product(name: "Holons", package: "swift-holons"),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: ".",
            exclude: [
                "api",
                "holons",
            ],
            sources: [
                "Sources/observability-cascade-swift",
                "gen/describe_generated.swift",
                "gen/swift/observability_cascade/v1",
                "gen/swift/relay/v1",
            ]
        ),
    ]
)
