// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "gudule-daemon-greeting-swift",
    platforms: [
        .macOS(.v13),
    ],
    products: [
        .library(
            name: "GreetingDaemonSwiftSupport",
            targets: ["GreetingDaemonSwiftSupport"]
        ),
        .library(
            name: "GreetingGenerated",
            targets: ["GreetingGenerated"]
        ),
        .executable(
            name: "gudule-daemon-greeting-swift",
            targets: ["GreetingDaemonSwift"]
        ),
    ],
    dependencies: [
        .package(path: "../../../sdk/swift-holons"),
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
    ],
    targets: [
        .target(
            name: "GreetingGenerated",
            dependencies: [
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: "gen/swift"
        ),
        .executableTarget(
            name: "GreetingDaemonSwift",
            dependencies: [
                "GreetingDaemonSwiftSupport",
                "GreetingGenerated",
                .product(name: "Holons", package: "swift-holons"),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: "Sources/GreetingDaemonSwift"
        ),
        .target(
            name: "GreetingDaemonSwiftSupport",
            dependencies: [
                "GreetingGenerated",
                .product(name: "Holons", package: "swift-holons"),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
            ],
            path: "Sources/GreetingDaemonSwiftSupport"
        ),
        .testTarget(
            name: "GreetingDaemonSwiftTests",
            dependencies: [
                "GreetingDaemonSwiftSupport",
                "GreetingGenerated",
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "NIOPosix", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: "Tests/GreetingDaemonSwiftTests"
        ),
    ]
)
