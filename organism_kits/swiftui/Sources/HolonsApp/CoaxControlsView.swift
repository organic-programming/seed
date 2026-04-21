import SwiftUI

public struct CoaxControlsView: View {
    @ObservedObject var coaxManager: CoaxManager
    @Binding var isShowingSettings: Bool

    public init(coaxManager: CoaxManager, isShowingSettings: Binding<Bool>) {
        self.coaxManager = coaxManager
        self._isShowingSettings = isShowingSettings
    }

    @available(*, deprecated, message: "Use init(coaxManager:isShowingSettings:)")
    public init(coaxServer: CoaxServer, isShowingSettings: Binding<Bool>) {
        self.init(coaxManager: coaxServer, isShowingSettings: isShowingSettings)
    }

    public var body: some View {
        VStack(alignment: .trailing, spacing: 8) {
            HStack(spacing: 8) {
                Toggle("COAX", isOn: $coaxManager.isEnabled)
                    .toggleStyle(.switch)
                    .font(.system(size: 12, weight: .medium, design: .monospaced))

                Button {
                    isShowingSettings = true
                } label: {
                    Image(systemName: "gearshape")
                        .font(.system(size: 14))
                        .foregroundStyle(isShowingSettings ? .primary : .secondary)
                        .padding(6)
                        .background(
                            Circle()
                                .fill(
                                    isShowingSettings
                                        ? Color.primary.opacity(0.08)
                                        : Color.clear
                                )
                        )
                }
                .buttonStyle(.plain)
            }

            Group {
                if let endpoint = coaxManager.serverStatus.endpoint {
                    HStack(alignment: .top, spacing: 6) {
                        Text(coaxManager.serverStatus.title + ":")
                            .font(.system(size: 11, weight: .semibold))
                            .foregroundStyle(.secondary)

                        Text(endpoint)
                            .font(.system(size: 11, weight: .regular, design: .monospaced))
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                            .multilineTextAlignment(.trailing)

                        Text(coaxManager.serverStatus.state.badgeTitle)
                            .font(.system(size: 9, weight: .bold, design: .monospaced))
                            .foregroundStyle(surfaceBadgeColor(coaxManager.serverStatus.state))
                    }
                    .frame(maxWidth: .infinity, alignment: .trailing)
                }
            }
            .frame(maxWidth: 520, alignment: .trailing)

            if let detail = coaxManager.statusDetail {
                Text(detail)
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(Color.orange)
                    .frame(width: 320, alignment: .trailing)
                    .multilineTextAlignment(.trailing)
            }
        }
    }
}

private func surfaceBadgeColor(_ state: CoaxSurfaceState) -> Color {
    switch state {
    case .off:
        return .secondary
    case .saved:
        return Color.gray
    case .announced:
        return Color.orange
    case .live:
        return Color.green
    case .error:
        return Color.red
    }
}
