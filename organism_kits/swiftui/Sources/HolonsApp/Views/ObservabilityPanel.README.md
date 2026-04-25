# ObservabilityPanel

`ObservabilityPanel(kit:)` is the four-tab SwiftUI surface for logs, metrics,
events, and relay/runtime settings. It inherits the host app's SwiftUI theme,
font, accent color, and control sizing.

Customise by wrapping the panel in the app's `.tint(...)`, `.font(...)`, and
container frame modifiers. The panel expects a medium-to-large surface and sets a
minimum size of 760 x 520 points.

