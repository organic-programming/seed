import GreetingKit
import Holons
import HolonsApp
import SwiftUI

#if os(macOS)
  import AppKit
#endif

@main
struct GabrielGreetingApp: App {
  @StateObject private var holonManager: GreetingHolonManager
  @StateObject private var coaxManager: CoaxManager

  init() {
    // Cross-SDK observability bootstrap: reads OP_OBS from the env
    // the launcher injected. Fail-fast on otel (v2) or unknown tokens
    // per OBSERVABILITY.md §Activation Layer 3. Safe no-op when OP_OBS
    // is empty.
    do {
      try checkEnv()
    } catch {
      FileHandle.standardError.write(
        "OP_OBS misconfigured: \(error)\n".data(using: .utf8) ?? Data())
    }
    _ = fromEnv(ObsConfig(slug: "gabriel-greeting-app"))
    current().emit(.instanceSpawned, payload: ["runtime": "swiftui"])
    current().logger("app").info("SwiftUI app starting")

    let holonManager = GreetingHolonManager()
    let settingsStore = FileSettingsStore.create(
      applicationId: "gabriel-greeting-app-swiftui",
      applicationName: "Gabriel Greeting App SwiftUI"
    )
    var turnOffCoax: (@MainActor @Sendable () -> Void)?
    let server = CoaxManager(
      providers: {
        [
          CoaxRpcServiceProvider(
            holonManager: holonManager,
            turnOffCoax: { turnOffCoax?() }
          ),
          GreetingAppServiceProvider(holon: holonManager),
        ]
      },
      registerDescribe: {
        let payload = try DescribeGenerated.staticDescribeResponse().serializedData()
          .base64EncodedString()
        try Describe.useStaticResponse(StaticDescribeResponse(payloadBase64: payload))
      },
      settingsStore: settingsStore,
      coaxDefaults: .standard(socketName: "gabriel-greeting-coax.sock")
    )
    turnOffCoax = { server.turnOffAfterRpc() }
    _holonManager = StateObject(wrappedValue: holonManager)
    _coaxManager = StateObject(wrappedValue: server)
    Task { @MainActor [server] in
      server.startIfEnabled()
    }
  }

  var body: some Scene {
    WindowGroup("Gabriel Greeting") {
      #if os(macOS)
        ContentView(holonManager: holonManager, coaxManager: coaxManager)
          .frame(minWidth: 800, minHeight: 600)
          .onAppear {
            DispatchQueue.main.async {
              revealAppWindow()
            }
          }
          .onDisappear {
            holonManager.stop()
            coaxManager.stop()
          }
          .onReceive(
            NotificationCenter.default.publisher(for: NSApplication.willTerminateNotification)
          ) { _ in
            holonManager.stop()
            coaxManager.stop()
          }
      #else
        ContentView(holonManager: holonManager, coaxManager: coaxManager)
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
