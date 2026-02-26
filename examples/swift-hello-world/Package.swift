// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "swift-hello-world",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .library(name: "SwiftHelloWorldCore", targets: ["SwiftHelloWorldCore"]),
        .executable(name: "swift-hello-world", targets: ["swift-hello-world"])
    ],
    dependencies: [
        .package(path: "../../sdk/swift-holons")
    ],
    targets: [
        .target(
            name: "SwiftHelloWorldCore",
            dependencies: [
                .product(name: "Holons", package: "swift-holons")
            ]
        ),
        .executableTarget(
            name: "swift-hello-world",
            dependencies: [
                "SwiftHelloWorldCore",
                .product(name: "Holons", package: "swift-holons")
            ]
        ),
        .testTarget(
            name: "SwiftHelloWorldTests",
            dependencies: ["SwiftHelloWorldCore"]
        )
    ]
)
