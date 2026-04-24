// swift-tools-version: 6.0
import Foundation
import PackageDescription

private let packageDirectory = URL(fileURLWithPath: #filePath)
    .resolvingSymlinksInPath()
    .deletingLastPathComponent()
private let swiftHolonsPath = URL(
    fileURLWithPath: "../../sdk/swift-holons",
    relativeTo: packageDirectory
)
    .standardizedFileURL
    .path

let package = Package(
    name: "HolonsApp",
    platforms: [.macOS(.v15), .iOS(.v18)],
    products: [
        .library(name: "HolonsApp", targets: ["HolonsApp"]),
    ],
    dependencies: [
        .package(path: swiftHolonsPath),
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
    ],
    targets: [
        .target(
            name: "HolonsApp",
            dependencies: [
                .product(name: "Holons", package: "swift-holons", condition: .when(platforms: [.macOS])),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: "Sources/HolonsApp",
            exclude: [
                "Views/CoaxControlsView.README.md",
                "Views/CoaxSettingsView.README.md",
                "Views/EventsView.README.md",
                "Views/HistogramChart.README.md",
                "Views/LogConsoleView.README.md",
                "Views/MetricsView.README.md",
                "Views/ObservabilityPanel.README.md",
                "Views/RelaySettingsView.README.md",
                "Views/SparklineView.README.md",
            ]
        ),
        .testTarget(
            name: "HolonsAppTests",
            dependencies: ["HolonsApp"],
            path: "Tests/HolonsAppTests"
        ),
    ]
)
