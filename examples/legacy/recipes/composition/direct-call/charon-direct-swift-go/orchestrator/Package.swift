// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "charon-direct-swift-go-orchestrator",
    products: [
        .executable(name: "charon-direct-swift-go-orchestrator", targets: ["charon-direct-swift-go-orchestrator"])
    ],
    targets: [
        .executableTarget(name: "charon-direct-swift-go-orchestrator")
    ]
)
