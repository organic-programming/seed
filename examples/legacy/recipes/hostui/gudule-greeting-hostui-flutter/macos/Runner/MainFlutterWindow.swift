import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)

    RegisterGeneratedPlugins(registry: flutterViewController)

    super.awakeFromNib()
    self.titleVisibility = .visible
    self.title = "Gudule \(resolveAssemblyFamily())"
  }

  private func resolveAssemblyFamily() -> String {
    if let family = ProcessInfo.processInfo.environment["OP_ASSEMBLY_DISPLAY_FAMILY"]?
      .trimmingCharacters(in: .whitespacesAndNewlines),
      !family.isEmpty
    {
      return family
    }

    if let family = ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]?
      .trimmingCharacters(in: .whitespacesAndNewlines),
      !family.isEmpty
    {
      return "\(family) (Flutter UI)"
    }

    if let resourceURL = Bundle.main.resourceURL,
      let entries = try? FileManager.default.contentsOfDirectory(
        at: resourceURL,
        includingPropertiesForKeys: nil,
        options: [.skipsHiddenFiles]
      )
    {
      for entry in entries {
        let binaryName = entry.lastPathComponent.replacingOccurrences(
          of: ".exe",
          with: "",
          options: .caseInsensitive
        )
        guard binaryName.hasPrefix("gudule-daemon-greeting-") else {
          continue
        }
        let variant = String(binaryName.dropFirst("gudule-daemon-greeting-".count))
        return "Greeting-Flutter-\(displayVariant(variant)) (Flutter UI)"
      }
    }

    return "Greeting-Flutter-Go (Flutter UI)"
  }

  private func displayVariant(_ variant: String) -> String {
    let overrides: [String: String] = [
      "cpp": "CPP",
      "js": "JS",
      "qt": "Qt",
    ]

    return variant
      .split(separator: "-")
      .map { token in
        let value = String(token)
        if let override = overrides[value] {
          return override
        }
        guard let first = value.first else {
          return value
        }
        return String(first).uppercased() + value.dropFirst()
      }
      .joined(separator: "-")
  }
}
