// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "GabrielGreetingApp",
    platforms: [.macOS(.v15), .iOS(.v18)],
    dependencies: [
        .package(path: "../../../sdk/swift-holons"),
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
    ],
    targets: [
        .executableTarget(
            name: "GabrielGreetingApp",
            dependencies: [
                .product(name: "Holons", package: "swift-holons", condition: .when(platforms: [.macOS])),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: "GabrielGreetingApp",
            exclude: ["Info.plist"]
        ),
    ]
)
