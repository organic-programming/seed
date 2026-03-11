import SwiftUI
#if os(macOS)
import AppKit
#endif

@main
struct GreetingSwiftUIApp: App {
    @StateObject private var daemon = DaemonProcess()

    var body: some Scene {
        WindowGroup {
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
