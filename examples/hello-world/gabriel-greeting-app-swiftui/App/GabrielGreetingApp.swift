import SwiftUI
import GreetingKit
import HolonsApp
import Holons
#if os(macOS)
import AppKit
#endif

@main
struct GabrielGreetingApp: App {
    @StateObject private var holon: HolonProcess
    @StateObject private var coaxServer: CoaxServer

    init() {
        let h = HolonProcess()
        let coaxServerBox = WeakCoaxServerBox()
        let server = CoaxServer(
            providers: {
                [
                    CoaxServiceProvider(organism: h, coaxServer: coaxServerBox.value!),
                    GreetingAppServiceProvider(holon: h),
                ]
            },
            registerDescribe: {
                let payload = try DescribeGenerated.staticDescribeResponse().serializedData().base64EncodedString()
                try Describe.useStaticResponse(StaticDescribeResponse(payloadBase64: payload))
            },
            coaxDefaults: .standard(socketName: "gabriel-greeting-coax.sock")
        )
        coaxServerBox.value = server
        _holon = StateObject(wrappedValue: h)
        _coaxServer = StateObject(wrappedValue: server)
        Task { @MainActor [server] in
            server.startIfEnabled()
        }
    }

    var body: some Scene {
        WindowGroup("Gabriel Greeting") {
#if os(macOS)
            ContentView(holon: holon, coaxServer: coaxServer)
                .frame(minWidth: 800, minHeight: 600)
                .onAppear {
                    DispatchQueue.main.async {
                        revealAppWindow()
                    }
                }
                .onDisappear {
                    holon.stop()
                    coaxServer.stop()
                }
                .onReceive(NotificationCenter.default.publisher(for: NSApplication.willTerminateNotification)) { _ in
                    holon.stop()
                    coaxServer.stop()
                }
#else
            ContentView(holon: holon, coaxServer: coaxServer)
#endif
        }
    }
}

private final class WeakCoaxServerBox {
    weak var value: CoaxServer?
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
