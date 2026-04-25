# CoaxSettingsView

`CoaxSettingsView(coaxManager:isPresented:)` is the public COAX settings panel.
It provides stock SwiftUI controls for transport, host, port, Unix socket path,
validation messages, and endpoint preview.

The view has no external theming dependency. Host apps customise it through
SwiftUI environment values, surrounding sheet chrome, and accent color.

