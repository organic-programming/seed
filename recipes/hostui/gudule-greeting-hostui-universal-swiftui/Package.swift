// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "GreetingSwiftUI",
    platforms: [
        .macOS(.v15),
        .iOS(.v18),
        .tvOS(.v18),
        .watchOS(.v11),
        .visionOS(.v2),
    ],
    dependencies: [
        .package(path: "../../daemons/gudule-daemon-greeting-swift"),
        .package(path: "../../../sdk/swift-holons"),
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
    ],
    targets: [
        .executableTarget(
            name: "GreetingSwiftUI",
            dependencies: [
                .product(
                    name: "Holons",
                    package: "swift-holons",
                    condition: .when(platforms: [.macOS])
                ),
                .product(
                    name: "GreetingDaemonSwiftSupport",
                    package: "gudule-daemon-greeting-swift",
                    condition: .when(platforms: [.macOS])
                ),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: "GreetingSwiftUI"
        ),
    ]
)
