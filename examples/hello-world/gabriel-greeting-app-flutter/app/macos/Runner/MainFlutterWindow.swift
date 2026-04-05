import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)
    self.title = "Gabriel Greeting"
    self.minSize = NSSize(width: 800, height: 600)

    if self.frame.size.width < self.minSize.width || self.frame.size.height < self.minSize.height {
      var resizedFrame = self.frame
      resizedFrame.size.width = max(resizedFrame.size.width, self.minSize.width)
      resizedFrame.size.height = max(resizedFrame.size.height, self.minSize.height)
      self.setFrame(resizedFrame, display: true)
      self.center()
    }

    RegisterGeneratedPlugins(registry: flutterViewController)

    super.awakeFromNib()
  }
}
