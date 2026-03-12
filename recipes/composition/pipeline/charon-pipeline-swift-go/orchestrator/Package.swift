// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "charon-pipeline-swift-go-orchestrator",
    products: [
        .executable(name: "charon-pipeline-swift-go-orchestrator", targets: ["charon-pipeline-swift-go-orchestrator"])
    ],
    targets: [
        .executableTarget(name: "charon-pipeline-swift-go-orchestrator")
    ]
)
