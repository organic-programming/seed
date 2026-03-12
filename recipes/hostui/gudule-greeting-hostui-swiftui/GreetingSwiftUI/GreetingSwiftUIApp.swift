import SwiftUI
#if os(macOS)
import AppKit
#endif

@main
struct GreetingSwiftUIApp: App {
    @StateObject private var daemon = DaemonProcess()
    private let assemblyFamily = ProcessInfo.processInfo.environment["OP_ASSEMBLY_DISPLAY_FAMILY"]
        ?? ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]
        ?? "Greeting-Swiftui-Go (SwiftUI)"

    var body: some Scene {
        WindowGroup("Gudule \(assemblyFamily)") {
#if os(macOS)
            ContentView(daemon: daemon)
                .frame(minWidth: 480, minHeight: 360)
                .onDisappear { daemon.stop() }
                .onReceive(NotificationCenter.default.publisher(for: NSApplication.willTerminateNotification)) { _ in
                    daemon.stop()
                }
#else
            ContentView(daemon: daemon)
#endif
        }
    }
}
