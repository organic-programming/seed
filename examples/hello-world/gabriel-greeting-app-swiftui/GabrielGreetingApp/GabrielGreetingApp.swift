import SwiftUI
#if os(macOS)
import AppKit
#endif

@main
struct GabrielGreetingApp: App {
    @StateObject private var daemon = DaemonProcess()

    var body: some Scene {
        WindowGroup("Gabriel Greeting") {
#if os(macOS)
            ContentView(daemon: daemon)
                .frame(minWidth: 800, minHeight: 600)
                .onAppear {
                    DispatchQueue.main.async {
                        revealAppWindow()
                    }
                }
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

#if os(macOS)
@MainActor
private func revealAppWindow() {
    NSApplication.shared.activate(ignoringOtherApps: true)

    guard let window = NSApplication.shared.windows.first else {
        return
    }

    let minimumSize = NSSize(width: 800, height: 600)
    var frame = window.frame
    let needsResize = frame.size.width < minimumSize.width || frame.size.height < minimumSize.height
    if needsResize {
        frame.size.width = max(frame.size.width, minimumSize.width)
        frame.size.height = max(frame.size.height, minimumSize.height)
        window.setFrame(frame, display: true)
        window.center()
    }

    window.makeKeyAndOrderFront(nil)
    window.orderFrontRegardless()
}
#endif
