# RelaySettingsView

`RelaySettingsView(kit:)` exposes the master gate, family gates, Prometheus HTTP
toggle, bound metrics address, and per-member relay overrides.

The member override control uses the required default / on / off tri-state. The
view inherits the app's SwiftUI theme and expects enough horizontal room for the
segmented member picker.

