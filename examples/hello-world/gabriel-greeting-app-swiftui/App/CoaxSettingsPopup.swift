import SwiftUI
import GreetingKit

private enum CoaxSettingsTab: String, CaseIterable, Identifiable {
    case server
    case relay
    case mcp

    var id: String { rawValue }

    var title: String {
        switch self {
        case .server:
            return "Server"
        case .relay:
            return "Relay"
        case .mcp:
            return "MCP"
        }
    }
}

struct CoaxSettingsPopup: View {
    @ObservedObject var coaxServer: CoaxServer
    @Binding var isPresented: Bool
    @State private var selectedTab: CoaxSettingsTab = .server

    private let sheetWidth: CGFloat = 780
    private let rowLabelWidth: CGFloat = 110

    private var sheetHeight: CGFloat {
        switch selectedTab {
        case .server:
            var height: CGFloat = coaxServer.serverTransport == .unix ? 560 : 588
            if coaxServer.serverPortValidationMessage != nil {
                height += 28
            }
            if coaxServer.serverTransportNote != nil {
                height += 42
            }
            return height
        case .relay:
            return 620
        case .mcp:
            switch coaxServer.mcpTransport {
            case .stdio:
                return 585
            case .streamableHTTP, .sse:
                return 635
            case .other:
                return 675
            }
        }
    }

    var body: some View {
        VStack(spacing: 0) {
            header
                .padding(.horizontal, 20)
                .padding(.top, 18)
                .padding(.bottom, 14)

            Divider()

            TabView(selection: $selectedTab) {
                tabContainer {
                    serverTab
                }
                .tag(CoaxSettingsTab.server)
                .tabItem { Text("Server") }

                tabContainer {
                    relayTab
                }
                .tag(CoaxSettingsTab.relay)
                .tabItem { Text("Relay") }

                tabContainer {
                    mcpTab
                }
                .tag(CoaxSettingsTab.mcp)
                .tabItem { Text("MCP") }
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 16)
        }
        .frame(minWidth: sheetWidth, idealWidth: sheetWidth, minHeight: sheetHeight, idealHeight: sheetHeight)
        .animation(.easeInOut(duration: 0.18), value: sheetHeight)
    }

    private var header: some View {
        HStack(alignment: .top, spacing: 18) {
            VStack(alignment: .leading, spacing: 4) {
                Text("COAX")
                    .font(.system(size: 22, weight: .semibold))

                Text("Configure server, relay, and MCP surfaces.")
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
            }

            Spacer(minLength: 12)

            HStack(spacing: 10) {
                Text("Enabled")
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(.secondary)

                Toggle("", isOn: $coaxServer.isEnabled)
                    .labelsHidden()
                    .toggleStyle(.switch)
            }

            Button("Done") {
                isPresented = false
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.regular)
        }
    }

    private var serverTab: some View {
        surfacePage(
            title: "Server",
            subtitle: "Expose the embedded runtime directly.",
            state: coaxServer.serverStatus.state,
            isEnabled: $coaxServer.serverEnabled
        ) {
            settingsSection("Connection") {
                pickerRow(
                    "Transport",
                    selection: $coaxServer.serverTransport,
                    values: CoaxServerTransport.allCases
                )

                switch coaxServer.serverTransport {
                case .tcp, .webSocket, .restSSE:
                    textFieldRow("Host", text: $coaxServer.serverHost, placeholder: "127.0.0.1")
                    textFieldRow(
                        "Port",
                        text: $coaxServer.serverPortText,
                        placeholder: String(coaxServer.serverTransport.defaultPort),
                        fieldWidth: 150
                    )
                case .unix:
                    textFieldRow(
                        "Socket path",
                        text: $coaxServer.serverUnixPath,
                        placeholder: "/tmp/gabriel-greeting-coax.sock"
                    )
                }
            }

            if let message = coaxServer.serverPortValidationMessage {
                noteRow(message)
            }

            if let note = coaxServer.serverTransportNote {
                noteRow(note)
            }

            previewSection(
                "Endpoint",
                value: coaxServer.serverStatus.endpoint ?? coaxServer.serverPreviewEndpoint
            )
        }
    }

    private var relayTab: some View {
        surfacePage(
            title: "Relay",
            subtitle: "Announce a client-facing surface through a COAX relay.",
            state: coaxServer.relayStatus.state,
            isEnabled: $coaxServer.relayEnabled
        ) {
            settingsSection("Connection") {
                pickerRow(
                    "Transport",
                    selection: $coaxServer.relayTransport,
                    values: CoaxRelayTransport.allCases
                )
                textFieldRow("Relay URL", text: $coaxServer.relayURL, placeholder: relayPlaceholder)
                secureFieldRow("Bearer token", text: $coaxServer.relayBearerToken, placeholder: "coax_relay_...")
            }

            noteRow(coaxServer.secretStorageNote)

            previewSection("Announced URL", value: coaxServer.relayPreviewEndpoint)
        }
    }

    private var mcpTab: some View {
        surfacePage(
            title: "MCP",
            subtitle: "Expose an MCP-facing surface alongside COAX.",
            state: coaxServer.mcpStatus.state,
            isEnabled: $coaxServer.mcpEnabled
        ) {
            settingsSection("Connection") {
                pickerRow(
                    "Transport",
                    selection: $coaxServer.mcpTransport,
                    values: CoaxMCPTransport.allCases
                )

                if coaxServer.mcpTransport == .stdio {
                    textFieldRow("Command", text: $coaxServer.mcpCommand, placeholder: "uvx my-mcp-server")
                } else {
                    textFieldRow("Endpoint", text: $coaxServer.mcpEndpoint, placeholder: mcpPlaceholder)
                    secureFieldRow("Bearer token", text: $coaxServer.mcpBearerToken, placeholder: "mcp_token_optional")

                    if coaxServer.mcpTransport == .other {
                        textFieldRow(
                            "Optional command",
                            text: $coaxServer.mcpCommand,
                            placeholder: "launcher or descriptor"
                        )
                    }
                }
            }

            noteRow(coaxServer.secretStorageNote)

            previewSection("Announced endpoint", value: coaxServer.mcpPreviewEndpoint)
        }
    }

    private var relayPlaceholder: String {
        switch coaxServer.relayTransport {
        case .restSSE:
            return "https://relay.example.com"
        case .webSocket:
            return "wss://relay.example.com/grpc"
        case .other:
            return "custom://relay.example.com"
        }
    }

    private var mcpPlaceholder: String {
        switch coaxServer.mcpTransport {
        case .stdio:
            return "uvx my-mcp-server"
        case .streamableHTTP:
            return "https://mcp.example.com"
        case .sse:
            return "https://mcp.example.com/sse"
        case .other:
            return "custom+mcp://surface"
        }
    }

    private func tabContainer<Content: View>(
        @ViewBuilder content: () -> Content
    ) -> some View {
        ScrollView(.vertical, showsIndicators: false) {
            content()
                .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }

    private func surfacePage<Content: View>(
        title: String,
        subtitle: String,
        state: CoaxSurfaceState,
        isEnabled: Binding<Bool>,
        @ViewBuilder content: () -> Content
    ) -> some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .top, spacing: 12) {
                VStack(alignment: .leading, spacing: 4) {
                    Text(title)
                        .font(.system(size: 18, weight: .semibold))

                    Text(subtitle)
                        .font(.system(size: 12))
                        .foregroundStyle(.secondary)
                }

                Spacer(minLength: 12)

                statusBadge(state)
            }

            Toggle("Enable this surface", isOn: isEnabled)
                .toggleStyle(.switch)

            content()
        }
        .frame(maxWidth: .infinity, alignment: .topLeading)
        .padding(.bottom, 6)
    }

    private func settingsSection<Content: View>(
        _ title: String,
        @ViewBuilder content: () -> Content
    ) -> some View {
        GroupBox {
            VStack(alignment: .leading, spacing: 12) {
                content()
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.top, 6)
        } label: {
            Text(title)
                .font(.system(size: 12, weight: .semibold))
        }
    }

    private func previewSection(
        _ title: String,
        value: String
    ) -> some View {
        GroupBox {
            VStack(alignment: .leading, spacing: 10) {
                Text(title)
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(.secondary)

                Text(value)
                    .font(.system(size: 13, weight: .regular, design: .monospaced))
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 14)
                    .background(
                        RoundedRectangle(cornerRadius: 10, style: .continuous)
                            .fill(Color(nsColor: .textBackgroundColor))
                    )
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.top, 6)
        }
    }

    private func noteRow(_ text: String) -> some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: "info.circle")
                .font(.system(size: 12))
                .foregroundStyle(.secondary)
                .padding(.top, 1)

            Text(text)
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private func pickerRow<Value: Hashable & Identifiable & CaseIterable>(
        _ title: String,
        selection: Binding<Value>,
        values: Value.AllCases
    ) -> some View where Value.AllCases: RandomAccessCollection, Value: RawRepresentable, Value.RawValue == String {
        formRow(title) {
            Picker("", selection: selection) {
                ForEach(Array(values)) { value in
                    Text(titleFor(value)).tag(value)
                }
            }
            .labelsHidden()
            .pickerStyle(.menu)
            .frame(minWidth: 220, idealWidth: 250, maxWidth: 280, alignment: .leading)
        }
    }

    private func textFieldRow(
        _ title: String,
        text: Binding<String>,
        placeholder: String,
        fieldWidth: CGFloat? = nil
    ) -> some View {
        formRow(title) {
            TextField(placeholder, text: text)
                .textFieldStyle(.roundedBorder)
                .frame(width: fieldWidth, alignment: .leading)
                .frame(maxWidth: fieldWidth == nil ? .infinity : nil, alignment: .leading)
        }
    }

    private func secureFieldRow(
        _ title: String,
        text: Binding<String>,
        placeholder: String
    ) -> some View {
        formRow(title) {
            SecureField(placeholder, text: text)
                .textFieldStyle(.roundedBorder)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    private func formRow<Content: View>(
        _ title: String,
        @ViewBuilder content: () -> Content
    ) -> some View {
        HStack(alignment: .center, spacing: 14) {
            Text(title)
                .font(.system(size: 13, weight: .medium))
                .frame(width: rowLabelWidth, alignment: .leading)

            content()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func statusBadge(_ state: CoaxSurfaceState) -> some View {
        Text(state.badgeTitle)
            .font(.system(size: 11, weight: .bold, design: .monospaced))
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .foregroundStyle(statusColor(for: state))
            .background(
                Capsule(style: .continuous)
                    .fill(statusColor(for: state).opacity(0.14))
            )
    }

    private func titleFor<Value>(_ value: Value) -> String where Value: RawRepresentable, Value.RawValue == String {
        switch value {
        case let transport as CoaxServerTransport:
            return transport.title
        case let transport as CoaxRelayTransport:
            return transport.title
        case let transport as CoaxMCPTransport:
            return transport.title
        default:
            return value.rawValue
        }
    }

    private func statusColor(for state: CoaxSurfaceState) -> Color {
        switch state {
        case .off:
            return .secondary
        case .saved:
            return .blue
        case .announced:
            return .orange
        case .live:
            return .green
        case .error:
            return .red
        }
    }
}
