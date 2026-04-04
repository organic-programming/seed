import SwiftUI
import GreetingKit

struct CoaxSettingsPopup: View {
    @ObservedObject var coaxServer: CoaxServer
    @Binding var isPresented: Bool

    private let sheetWidth: CGFloat = 780
    private let rowLabelWidth: CGFloat = 110

    private var sheetHeight: CGFloat {
        var height: CGFloat = coaxServer.serverTransport == .unix ? 560 : 588
        if coaxServer.serverPortValidationMessage != nil {
            height += 28
        }
        if coaxServer.serverTransportNote != nil {
            height += 42
        }
        return height
    }

    var body: some View {
        VStack(spacing: 0) {
            header
                .padding(.horizontal, 20)
                .padding(.top, 18)
                .padding(.bottom, 14)

            Divider()

            ScrollView(.vertical, showsIndicators: false) {
                serverPage
                    .frame(maxWidth: .infinity, alignment: .topLeading)
                    .padding(.horizontal, 20)
                    .padding(.vertical, 16)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        }
        .frame(minWidth: sheetWidth, idealWidth: sheetWidth, minHeight: sheetHeight, idealHeight: sheetHeight)
        .animation(.easeInOut(duration: 0.18), value: sheetHeight)
    }

    private var header: some View {
        HStack(alignment: .top, spacing: 18) {
            VStack(alignment: .leading, spacing: 4) {
                Text("COAX")
                    .font(.system(size: 22, weight: .semibold))

                Text("Configure the server surface.")
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

    private var serverPage: some View {
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
        if let transport = value as? CoaxServerTransport {
            return transport.title
        }
        return value.rawValue
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
