// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "swift-holons",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .library(
            name: "Holons",
            targets: ["Holons"]
        ),
        .executable(
            name: "echo-client",
            targets: ["EchoClient"]
        ),
        .executable(
            name: "echo-server",
            targets: ["EchoServer"]
        ),
        .executable(
            name: "holon-rpc-server",
            targets: ["HolonRPCServer"]
        ),
        .executable(
            name: "protoless-describe-fixture",
            targets: ["ProtoLessDescribeFixture"]
        )
    ],
    dependencies: [
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
        .package(url: "https://github.com/apple/swift-log.git", from: "1.6.0"),
    ],
    targets: [
        .target(
            name: "Holons",
            dependencies: [
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "NIOPosix", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
                .product(name: "Logging", package: "swift-log"),
            ]
        ),
        .executableTarget(
            name: "EchoClient",
            dependencies: ["Holons"],
            path: "Sources/echo-client"
        ),
        .executableTarget(
            name: "EchoServer",
            dependencies: ["Holons"],
            path: "Sources/echo-server"
        ),
        .executableTarget(
            name: "HolonRPCServer",
            dependencies: ["Holons"],
            path: "Sources/holon-rpc-server"
        ),
        .executableTarget(
            name: "ProtoLessDescribeFixture",
            dependencies: ["Holons"],
            path: "Sources/protoless-describe-fixture"
        ),
        .testTarget(
            name: "HolonsTests",
            dependencies: [
                "Holons",
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "NIOPosix", package: "swift-nio"),
            ],
            exclude: ["Fixtures"]
        )
    ]
)
