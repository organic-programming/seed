// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "charon-fanout-swift-go-orchestrator",
    products: [
        .executable(name: "charon-fanout-swift-go-orchestrator", targets: ["charon-fanout-swift-go-orchestrator"])
    ],
    targets: [
        .executableTarget(name: "charon-fanout-swift-go-orchestrator")
    ]
)
