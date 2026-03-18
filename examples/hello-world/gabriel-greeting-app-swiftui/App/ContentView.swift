import SwiftUI
import GreetingKit

struct ContentView: View {
    @ObservedObject var holon: HolonProcess
    private let inputColumnWidth: CGFloat = 300
    private let contentSpacing: CGFloat = 32
    private let languagePickerWidth: CGFloat = 220
    private let holonSlugWidth: CGFloat = 360
    @State private var languages: [Language] = []
    @State private var selectedCode: String = ""
    @State private var userName: String = ""
    @State private var greeting: String = ""
    @State private var error: String?
    @State private var isLoading = true
    @State private var isGreeting = false
    @FocusState private var isNameFieldFocused: Bool

    private func normalizedTransportSelection(_ value: String) -> String {
        switch value.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
        case "", "auto", "stdio", "stdio://":
            return "stdio"
        case "mem", "mem://":
            return "mem"
        case "unix", "unix://":
            return "unix"
        case "tcp", "tcp://":
            return "tcp"
        case "rest+sse", "http", "http://", "https", "https://", "sse", "rest":
            return "rest+sse"
        default:
            return "stdio"
        }
    }

    private var transportSelection: Binding<String> {
        Binding(
            get: { normalizedTransportSelection(holon.transport) },
            set: { holon.transport = $0 }
        )
    }

    private var statusTitle: String {
        if isLoading {
            return "Starting holon..."
        }
        if holon.isRunning {
            return "Ready"
        }
        return "Offline"
    }

    private var statusColor: Color {
        if isLoading {
            return Color.orange
        }
        if holon.isRunning {
            return Color.green
        }
        return Color.red
    }

    var body: some View {
        VStack(spacing: 0) {
            topHeaderArea
            VStack(spacing: 24) {
                HStack(alignment: .center, spacing: contentSpacing) {
                    inputColumn
                        .frame(width: inputColumnWidth)

                    bubbleColumn
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)

                HStack(alignment: .center, spacing: contentSpacing) {
                    Color.clear
                        .frame(width: inputColumnWidth, height: 1)

                    HStack {
                        Spacer(minLength: 0)
                        languagePicker
                        Spacer(minLength: 0)
                    }
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)
            .padding(32)
        }
        .frame(minWidth: 800, minHeight: 600, alignment: .topLeading)
        .background(Color(nsColor: .windowBackgroundColor).ignoresSafeArea())
        .animation(.easeInOut(duration: 0.2), value: error)
        .task {
            let normalized = normalizedTransportSelection(holon.transport)
            if holon.transport != normalized {
                holon.transport = normalized
            }
            await loadLanguages()
        }
    }

    private func loadLanguages() async {
        isLoading = true
        error = nil
        greeting = ""
        languages = []
        selectedCode = ""
        await holon.start()
        guard holon.isRunning else {
            let detail = holon.connectionError ?? "Holon did not become ready"
            error = "Failed to load languages: \(detail)"
            isLoading = false
            return
        }
        let retryDelays: [UInt64] = normalizedTransportSelection(holon.transport) == "stdio"
            ? [0, 80_000_000, 180_000_000]
            : [120_000_000, 300_000_000, 600_000_000]

        for (attempt, delay) in retryDelays.enumerated() {
            do {
#if os(macOS)
                if delay > 0 {
                    try await Task.sleep(nanoseconds: delay)
                }
#endif
                languages = try await holon.listLanguages()
                selectedCode = languages.first(where: { $0.code == "en" })?.code ?? languages.first?.code ?? ""
                error = nil
                isLoading = false
                if !selectedCode.isEmpty {
                    Task { await greet(code: selectedCode) }
                }
                return
            } catch {
                if attempt == retryDelays.count - 1 {
                    let detail = holon.connectionError ?? error.localizedDescription
                    self.error = "Failed to load languages: \(detail)"
                    isLoading = false
                }
            }
        }
    }

    private func greet(code: String) async {
        guard !code.isEmpty else { return }
        isGreeting = true
        do {
            greeting = try await holon.sayHello(name: userName, langCode: code)
            error = nil
        } catch {
            self.error = "Greeting failed: \(error.localizedDescription)"
        }
        isGreeting = false
    }

    private var topHeaderArea: some View {
        HStack(alignment: .top) {
            VStack(alignment: .leading, spacing: 6) {
                Picker("", selection: $holon.selectedHolon) {
                    if holon.availableHolons.isEmpty {
                        Text("Loading holons...").tag(nil as GabrielHolonIdentity?)
                    } else {
                        ForEach(holon.availableHolons) { identity in
                            Text(identity.displayName).tag(identity as GabrielHolonIdentity?)
                        }
                    }
                }
                .labelsHidden()
                .frame(width: 250)
                .onChange(of: holon.selectedHolon?.id) {
                    Task { await loadLanguages() }
                }

                if let slug = holon.selectedHolon?.slug {
                    Text(slug)
                        .font(.system(size: 11, weight: .regular, design: .monospaced))
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                        .lineLimit(2)
                        .frame(width: holonSlugWidth, alignment: .leading)
                }
            }

            Spacer()

            VStack(alignment: .trailing, spacing: 6) {
                HStack(spacing: 8) {
                    Text("mode:")
                        .font(.system(size: 14, weight: .medium))
                        .foregroundStyle(.primary)

                    Picker("", selection: transportSelection) {
                        Text("mem").tag("mem")
                        Text("stdio").tag("stdio")
                        Text("unix").tag("unix")
                        Text("tcp").tag("tcp")
                        Text("rest+sse").tag("rest+sse")
                    }
                    .labelsHidden()
                    .frame(width: 140)
                    .onChange(of: holon.transport) {
                        Task { await loadLanguages() }
                    }
                }

                HStack(spacing: 8) {
                    Text(statusTitle)
                        .font(.system(size: 14, weight: .medium))
                        .foregroundStyle(.primary)
                    Circle()
                        .fill(statusColor)
                        .frame(width: 10, height: 10)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .center)
        .padding(.horizontal, 32)
        .padding(.vertical, 20)
        .background(Color(nsColor: .controlBackgroundColor))
        .overlay(
            Rectangle().frame(height: 1).foregroundColor(Color.primary.opacity(0.06)),
            alignment: .bottom
        )
    }

    private var inputColumn: some View {
        VStack(alignment: .leading, spacing: 5) {
            if #available(macOS 13.0, *) {
                TextField("", text: $userName, axis: .vertical)
                    .lineLimit(4, reservesSpace: true)
                    .textFieldStyle(.roundedBorder)
                    .focused($isNameFieldFocused)
                    .onChange(of: userName) {
                        Task { await greet(code: selectedCode) }
                    }
                    .frame(width: inputColumnWidth)
            } else {
                TextEditor(text: $userName)
                    .focused($isNameFieldFocused)
                    .onChange(of: userName) {
                        Task { await greet(code: selectedCode) }
                    }
                    .frame(width: inputColumnWidth, height: 100)
                    .cornerRadius(6)
            }
        }
    }

    private var languagePicker: some View {
        Picker("", selection: $selectedCode) {
            if isLoading {
                Text("Loading...").tag("")
            } else {
                ForEach(languages) { language in
                    Text("\(language.native) (\(language.name))").tag(language.code)
                }
            }
        }
        .labelsHidden()
        .frame(width: languagePickerWidth)
        .onChange(of: selectedCode) {
            Task { await greet(code: selectedCode) }
        }
    }

    private var bubbleColumn: some View {
        GeometryReader { geometry in
            ZStack {
                LeftPointerBubble()
                    .fill(Color.clear)

                LeftPointerBubble()
                    .stroke(
                        Color.primary.opacity(0.4),
                        style: StrokeStyle(lineWidth: 1.5, lineCap: .round, lineJoin: .round, dash: [0.1, 5])
                    )

                if let connectionError = holon.connectionError {
                    VStack(alignment: .leading, spacing: 12) {
                        HStack {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundColor(Color(red: 1.0, green: 0.4, blue: 0.4))
                                .font(.system(size: 20))
                            Text("Holon Offline")
                                .font(.system(size: 18, weight: .bold))
                                .foregroundColor(Color(red: 1.0, green: 0.4, blue: 0.4))
                        }
                        Text(connectionError)
                            .font(.system(size: 13, weight: .regular, design: .monospaced))
                            .foregroundColor(Color.primary.opacity(0.85))
                            .textSelection(.enabled)
                    }
                    .padding(24)
                } else if let error {
                    VStack(alignment: .leading, spacing: 12) {
                        HStack {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundColor(Color(red: 1.0, green: 0.4, blue: 0.4))
                                .font(.system(size: 20))
                            Text("Error")
                                .font(.system(size: 18, weight: .bold))
                                .foregroundColor(Color(red: 1.0, green: 0.4, blue: 0.4))
                        }
                        Text(error)
                            .font(.system(size: 13, weight: .regular, design: .monospaced))
                            .foregroundColor(Color.primary.opacity(0.85))
                            .textSelection(.enabled)
                    }
                    .padding(24)
                } else {
                    Text(greeting)
                        .font(.system(size: 42, weight: .medium))
                        .foregroundColor(.primary)
                        .lineLimit(nil)
                        .multilineTextAlignment(.center)
                        .padding(.leading, 20)
                }
            }
            .frame(width: geometry.size.width, height: geometry.size.height, alignment: .center)
        }
    }
}

struct LeftPointerBubble: Shape {
    var cornerRadius: CGFloat = 16
    var pointerSize: CGFloat = 14

    func path(in rect: CGRect) -> Path {
        var path = Path()
        let minX = pointerSize
        let maxX = rect.maxX
        let minY = rect.minY
        let maxY = rect.maxY
        let pointerCenterY = rect.midY

        path.move(to: CGPoint(x: minX + cornerRadius, y: minY))
        path.addLine(to: CGPoint(x: maxX - cornerRadius, y: minY))
        path.addArc(
            center: CGPoint(x: maxX - cornerRadius, y: minY + cornerRadius),
            radius: cornerRadius,
            startAngle: .degrees(-90),
            endAngle: .degrees(0),
            clockwise: false
        )
        path.addLine(to: CGPoint(x: maxX, y: maxY - cornerRadius))
        path.addArc(
            center: CGPoint(x: maxX - cornerRadius, y: maxY - cornerRadius),
            radius: cornerRadius,
            startAngle: .degrees(0),
            endAngle: .degrees(90),
            clockwise: false
        )
        path.addLine(to: CGPoint(x: minX + cornerRadius, y: maxY))
        path.addArc(
            center: CGPoint(x: minX + cornerRadius, y: maxY - cornerRadius),
            radius: cornerRadius,
            startAngle: .degrees(90),
            endAngle: .degrees(180),
            clockwise: false
        )
        path.addLine(to: CGPoint(x: minX, y: pointerCenterY + pointerSize))
        path.addLine(to: CGPoint(x: 0, y: pointerCenterY))
        path.addLine(to: CGPoint(x: minX, y: pointerCenterY - pointerSize))
        path.addLine(to: CGPoint(x: minX, y: minY + cornerRadius))
        path.addArc(
            center: CGPoint(x: minX + cornerRadius, y: minY + cornerRadius),
            radius: cornerRadius,
            startAngle: .degrees(180),
            endAngle: .degrees(270),
            clockwise: false
        )

        return path
    }
}
