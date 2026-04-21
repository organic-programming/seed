import SwiftUI

public struct CoaxSettingsView: View {
    @ObservedObject var coaxManager: CoaxManager
    @Binding var isPresented: Bool

    private let sheetWidth: CGFloat = 780
    private let rowLabelWidth: CGFloat = 110

    public init(coaxManager: CoaxManager, isPresented: Binding<Bool>) {
        self.coaxManager = coaxManager
        self._isPresented = isPresented
    }

    @available(*, deprecated, message: "Use init(coaxManager:isPresented:)")
    public init(coaxServer: CoaxServer, isPresented: Binding<Bool>) {
        self.init(coaxManager: coaxServer, isPresented: isPresented)
    }

    private var sheetHeight: CGFloat {
        var height: CGFloat = coaxManager.serverTransport == .unix ? 560 : 588
        if coaxManager.serverPortValidationMessage != nil {
            height += 28
        }
        return height
    }

    public var body: some View {
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

            Button("Done") {
                isPresented = false
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.regular)
        }
    }

    private var serverPage: some View {
        VStack(alignment: .leading, spacing: 16) {
            settingsSection("Connection") {
                pickerRow(
                    "Transport",
                    selection: $coaxManager.serverTransport,
                    values: CoaxServerTransport.allCases
                )

                switch coaxManager.serverTransport {
                case .tcp:
                    textFieldRow("Host", text: $coaxManager.serverHost, placeholder: "127.0.0.1")
                    textFieldRow(
                        "Port",
                        text: $coaxManager.serverPortText,
                        placeholder: String(coaxManager.serverTransport.defaultPort),
                        fieldWidth: 150
                    )
                case .unix:
                    textFieldRow(
                        "Socket path",
                        text: $coaxManager.serverUnixPath,
                        placeholder: coaxManager.defaultUnixPath
                    )
                }
            }

            if let message = coaxManager.serverPortValidationMessage {
                noteRow(message)
            }

            previewSection(
                "Endpoint",
                value: coaxManager.serverStatus.endpoint ?? coaxManager.serverPreviewEndpoint
            )
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

    private func titleFor<Value>(_ value: Value) -> String where Value: RawRepresentable, Value.RawValue == String {
        if let transport = value as? CoaxServerTransport {
            return transport.title
        }
        return value.rawValue
    }
}
