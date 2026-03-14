import SwiftUI

struct ContentView: View {
    @ObservedObject var daemon: DaemonProcess
    private let assemblyFamily = ProcessInfo.processInfo.environment["OP_ASSEMBLY_DISPLAY_FAMILY"]
        ?? ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]
        ?? "Greeting-Swiftui-Go"
    private let inputColumnWidth: CGFloat = 300
    private let contentSpacing: CGFloat = 32
    private let languagePickerWidth: CGFloat = 220
    private let daemonSlugWidth: CGFloat = 360
    @State private var languages: [Language] = []
    @State private var selectedCode: String = ""
    @State private var userName: String = "World"
    @State private var greeting: String = ""
    @State private var error: String?
    @State private var isLoading = true
    @State private var isGreeting = false
    @FocusState private var isNameFieldFocused: Bool

    private var selectedLanguage: Language? {
        languages.first(where: { $0.code == selectedCode })
    }



    private var statusTitle: String {
        if isLoading {
            return "Starting daemon..."
        }
        if daemon.isRunning {
            return "Ready"
        }
        return "Offline"
    }

    private var statusColor: Color {
        if isLoading {
            return Color.orange
        }
        if daemon.isRunning {
            return Color.green
        }
        return Color.red
    }

    // formattedTitle removed as per mockup
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
        .frame(minWidth: 800, minHeight: 500, alignment: .topLeading)
        .background(Color(red: 0.1, green: 0.1, blue: 0.1).ignoresSafeArea())
        .animation(.easeInOut(duration: 0.2), value: error)
        .task { await loadLanguages() }
    }

    private func loadLanguages() async {
        isLoading = true
        error = nil
        greeting = ""
        languages = []
        selectedCode = ""
        await daemon.start()
        guard daemon.isRunning else {
            let detail = daemon.connectionError ?? "Daemon did not become ready"
            self.error = "Failed to load languages: \(detail)"
            self.isLoading = false
            return
        }
        let retryDelays: [UInt64] = daemon.transport == "stdio"
            ? [0, 80_000_000, 180_000_000]
            : [120_000_000, 300_000_000, 600_000_000]

        for (attempt, delay) in retryDelays.enumerated() {
            do {
#if os(macOS)
                if delay > 0 {
                    try await Task.sleep(nanoseconds: delay)
                }
#endif
                languages = try await daemon.listLanguages()
                selectedCode = languages.first(where: { $0.code == "en" })?.code ?? languages.first?.code ?? ""
                error = nil
                isLoading = false
                if !selectedCode.isEmpty && !userName.isEmpty {
                    Task { await greet(code: selectedCode) }
                }
                return
            } catch {
                if attempt == 2 {
                    let detail = daemon.connectionError ?? error.localizedDescription
                    self.error = "Failed to load languages: \(detail)"
                    self.isLoading = false
                }
            }
        }
    }

    private func greet(code: String) async {
        guard !code.isEmpty else { return }
        isGreeting = true
        do {
            greeting = try await daemon.sayHello(name: userName, langCode: code)
            error = nil
        } catch {
            self.error = "Greeting failed: \(error.localizedDescription)"
        }
        isGreeting = false
    }

    private var topHeaderArea: some View {
        HStack(alignment: .top) {
            VStack(alignment: .leading, spacing: 6) {
                Picker("", selection: $daemon.selectedDaemon) {
                    if daemon.availableDaemons.isEmpty {
                        Text("Loading daemons...").tag(nil as GreetingDaemonIdentity?)
                    } else {
                        ForEach(daemon.availableDaemons) { d in
                            Text(d.displayName).tag(d as GreetingDaemonIdentity?)
                        }
                    }
                }
                .labelsHidden()
                .frame(width: 250)
                .onChange(of: daemon.selectedDaemon?.id) {
                    Task { await loadLanguages() }
                }

                if let slug = daemon.selectedDaemon?.slug {
                    Text(slug)
                        .font(.system(size: 11, weight: .regular, design: .monospaced))
                        .foregroundStyle(Color.white.opacity(0.7))
                        .textSelection(.enabled)
                        .lineLimit(2)
                        .frame(width: daemonSlugWidth, alignment: .leading)
                }
            }
            
            Spacer()
            
            VStack(alignment: .trailing, spacing: 6) {
                // Mode Picker
                HStack(spacing: 8) {
                    Text("mode:")
                        .font(.system(size: 14, weight: .medium))
                        .foregroundStyle(.white)
                    
                    Picker("", selection: $daemon.transport) {
                        Text("mem").tag("mem")
                        Text("stdio").tag("stdio")
                        Text("unix").tag("unix")
                        Text("tcp").tag("tcp")
                        Text("rest+sse").tag("rest+sse")
                    }
                    .labelsHidden()
                    .frame(width: 140)
                    .onChange(of: daemon.transport) {
                        Task { await loadLanguages() }
                    }
                }
                
                // Status Indicator
                HStack(spacing: 8) {
                    Text(statusTitle)
                        .font(.system(size: 14, weight: .medium))
                        .foregroundStyle(Color.white)
                    Circle()
                        .fill(statusColor)
                        .frame(width: 10, height: 10)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .center)
        .padding(.horizontal, 32)
        .padding(.vertical, 20)
        .background(Color(red: 0.13, green: 0.13, blue: 0.13))
        .overlay(
            Rectangle().frame(height: 1).foregroundColor(Color.white.opacity(0.06)),
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
                ForEach(languages) { lang in
                    Text("\(lang.native) (\(lang.name))").tag(lang.code)
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
        GeometryReader { geo in
            ZStack {
                // Background & Border Shapes
                LeftPointerBubble()
                    .fill(Color.clear)
                
                LeftPointerBubble()
                    .stroke(Color.white.opacity(0.4), style: StrokeStyle(lineWidth: 1.5, lineCap: .round, lineJoin: .round, dash: [0.1, 5]))

                if let err = daemon.connectionError {
                    VStack(alignment: .leading, spacing: 12) {
                        HStack {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundColor(Color(red: 1.0, green: 0.4, blue: 0.4))
                                .font(.system(size: 20))
                            Text("Daemon Offline")
                                .font(.system(size: 18, weight: .bold))
                                .foregroundColor(Color(red: 1.0, green: 0.4, blue: 0.4))
                        }
                        Text(err)
                            .font(.system(size: 13, weight: .regular, design: .monospaced))
                            .foregroundColor(Color.white.opacity(0.85))
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
                            .foregroundColor(Color.white.opacity(0.85))
                            .textSelection(.enabled)
                    }
                    .padding(24)
                } else if isLoading {
                    VStack(spacing: 12) {
                        ProgressView()
                            .progressViewStyle(.circular)
                            .tint(.white.opacity(0.85))
                        Text("Connecting...")
                            .font(.system(size: 18, weight: .medium))
                            .foregroundColor(Color.white.opacity(0.8))
                    }
                    .padding(24)
                } else {
                    Text(greeting)
                        .font(.system(size: 42, weight: .medium))
                        .foregroundColor(Color.white)
                        .lineLimit(nil)
                        .multilineTextAlignment(.center)
                        .padding(.leading, 20)
                }
            }
            .frame(width: geo.size.width, height: geo.size.height, alignment: .center)
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
        path.addArc(center: CGPoint(x: maxX - cornerRadius, y: minY + cornerRadius), radius: cornerRadius, startAngle: .degrees(-90), endAngle: .degrees(0), clockwise: false)
        path.addLine(to: CGPoint(x: maxX, y: maxY - cornerRadius))
        path.addArc(center: CGPoint(x: maxX - cornerRadius, y: maxY - cornerRadius), radius: cornerRadius, startAngle: .degrees(0), endAngle: .degrees(90), clockwise: false)
        path.addLine(to: CGPoint(x: minX + cornerRadius, y: maxY))
        path.addArc(center: CGPoint(x: minX + cornerRadius, y: maxY - cornerRadius), radius: cornerRadius, startAngle: .degrees(90), endAngle: .degrees(180), clockwise: false)
        path.addLine(to: CGPoint(x: minX, y: pointerCenterY + pointerSize))
        path.addLine(to: CGPoint(x: 0, y: pointerCenterY))
        path.addLine(to: CGPoint(x: minX, y: pointerCenterY - pointerSize))
        path.addLine(to: CGPoint(x: minX, y: minY + cornerRadius))
        path.addArc(center: CGPoint(x: minX + cornerRadius, y: minY + cornerRadius), radius: cornerRadius, startAngle: .degrees(180), endAngle: .degrees(270), clockwise: false)
        
        return path
    }
}

/// A language returned by the daemon.
struct Language: Identifiable {
    let code: String
    let name: String
    let native: String
    var id: String { code }
}
