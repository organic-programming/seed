// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "gabriel-greeting-swift",
    platforms: [.macOS(.v13)],
    products: [
        .library(name: "GabrielGreeting", targets: ["GabrielGreeting"]),
        .library(name: "GabrielGreetingServer", targets: ["GabrielGreetingServer"]),
        .executable(name: "gabriel-greeting-swift", targets: ["gabriel-greeting-swift"]),
    ],
    dependencies: [
        .package(path: "../../../sdk/swift-holons"),
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
    ],
    targets: [
        .target(
            name: "GabrielGreeting",
            dependencies: [
                .product(name: "Holons", package: "swift-holons"),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIO", package: "swift-nio"),
                .product(name: "NIOConcurrencyHelpers", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: ".",
            exclude: [
                "Sources/GabrielGreetingServer",
                "Sources/gabriel-greeting-swift",
                "Tests",
                "api",
                "README.md",
            ],
            sources: [
                "Sources/GabrielGreeting",
                "gen/describe_generated.swift",
                "gen/swift/greeting/v1",
            ]
        ),
        .target(
            name: "GabrielGreetingServer",
            dependencies: [
                "GabrielGreeting",
                .product(name: "Holons", package: "swift-holons"),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIO", package: "swift-nio"),
            ],
            path: "Sources/GabrielGreetingServer"
        ),
        .executableTarget(
            name: "gabriel-greeting-swift",
            dependencies: [
                "GabrielGreeting",
                "GabrielGreetingServer",
            ],
            path: "Sources/gabriel-greeting-swift"
        ),
        .testTarget(
            name: "GabrielGreetingTests",
            dependencies: ["GabrielGreeting"],
            path: "Tests/GabrielGreetingTests"
        ),
        .testTarget(
            name: "GabrielGreetingServerTests",
            dependencies: [
                "GabrielGreeting",
                "GabrielGreetingServer",
                .product(name: "Holons", package: "swift-holons"),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOPosix", package: "swift-nio"),
            ],
            path: "Tests/GabrielGreetingServerTests"
        ),
    ]
)
