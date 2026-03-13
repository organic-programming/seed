import SwiftUI

struct ContentView: View {
    @ObservedObject var daemon: DaemonProcess
    private let assemblyFamily = ProcessInfo.processInfo.environment["OP_ASSEMBLY_DISPLAY_FAMILY"]
        ?? ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]
        ?? "Greeting-Swiftui-Go"
    @State private var languages: [Language] = []
    @State private var selectedCode: String = ""
    @State private var userName: String = "World!"
    @State private var greeting: String = ""
    @State private var error: String?
    @State private var isLoading = true
    @State private var isGreeting = false
    @FocusState private var isNameFieldFocused: Bool

    private var selectedLanguage: Language? {
        languages.first(where: { $0.code == selectedCode })
    }

    private var trimmedUserName: String {
        userName.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var canGreet: Bool {
        !selectedCode.isEmpty && !trimmedUserName.isEmpty && !isLoading && !isGreeting
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

    private var formattedTitle: String {
        var family = assemblyFamily
        if let idx = family.firstIndex(of: " ") {
            family = String(family[..<idx])
        }
        let parts = family.split(separator: "-").map(String.init)
        if parts.count >= 3 {
            let ui = parts[1].prefix(1).uppercased() + parts[1].dropFirst()
            let logic = parts[2].prefix(1).uppercased() + parts[2].dropFirst()
            return "Gudule : \(ui) / \(logic)"
        }
        return "Gudule : \(family)"
    }

    var body: some View {
        VStack(spacing: 0) {
            topHeaderArea
            
            HStack(alignment: .top, spacing: 32) {
                leftColumn
                rightColumn
            }
            .padding(32)
            
            bottomActionBar
        }
        .frame(minWidth: 800, minHeight: 500, alignment: .topLeading)
        .background(Color(red: 0.1, green: 0.1, blue: 0.1).ignoresSafeArea())
        .animation(.spring(), value: greeting)
        .animation(.easeInOut(duration: 0.2), value: error)
        .task { await loadLanguages() }
    }

    private func loadLanguages() async {
        isLoading = true
        error = nil
        daemon.start()

        for attempt in 0..<3 {
            do {
#if os(macOS)
                if attempt > 0 {
                    try await Task.sleep(nanoseconds: 500_000_000)
                } else {
                    try await Task.sleep(nanoseconds: 300_000_000)
                }
#endif
                languages = try await daemon.listLanguages()
                selectedCode = languages.first(where: { $0.code == "en" })?.code ?? languages.first?.code ?? ""
                error = nil
                isLoading = false
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
        guard !code.isEmpty, !trimmedUserName.isEmpty else { return }
        isGreeting = true
        do {
            greeting = try await daemon.sayHello(name: trimmedUserName, langCode: code)
            error = nil
        } catch {
            self.error = "Greeting failed: \(error.localizedDescription)"
        }
        isGreeting = false
    }

    private var topHeaderArea: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(formattedTitle)
                    .font(.system(size: 26, weight: .bold, design: .default))
                    .foregroundStyle(.white)
                
                Spacer()
            }
            
            Text("Connected to \(daemon.daemonBinaryName)")
                .font(.system(size: 13, weight: .regular, design: .default))
                .foregroundStyle(Color.white.opacity(0.55))
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 32)
        .padding(.vertical, 20)
        .background(Color(red: 0.13, green: 0.13, blue: 0.13))
        .overlay(
            Rectangle().frame(height: 1).foregroundColor(Color.white.opacity(0.06)),
            alignment: .bottom
        )
    }

    private var leftColumn: some View {
        VStack(alignment: .leading, spacing: 32) {
            // Status Indicator
            HStack(spacing: 8) {
                Circle()
                    .fill(statusColor)
                    .frame(width: 10, height: 10)
                Text(statusTitle)
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Color.white.opacity(0.85))
            }
            
            // Input Controls
            VStack(alignment: .leading, spacing: 20) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Mode")
                        .font(.system(size: 11, weight: .semibold))
                        .foregroundStyle(Color.white.opacity(0.5))
                        .textCase(.uppercase)
                    
                    Picker("", selection: $daemon.transport) {
                        Text("mem").tag("mem")
                        Text("stdio").tag("stdio")
                        Text("unix").tag("unix")
                        Text("tcp").tag("tcp")
                        Text("rest+sse").tag("rest+sse")
                    }
                    .labelsHidden()
                    .frame(maxWidth: .infinity)
                    .onChange(of: daemon.transport) { _ in
                        Task { await loadLanguages() }
                    }
                }

                VStack(alignment: .leading, spacing: 8) {
                    Text("Language")
                        .font(.system(size: 11, weight: .semibold))
                        .foregroundStyle(Color.white.opacity(0.5))
                        .textCase(.uppercase)
                    
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
                    .frame(maxWidth: .infinity)
                }

                VStack(alignment: .leading, spacing: 8) {
                    Text("Name")
                        .font(.system(size: 11, weight: .semibold))
                        .foregroundStyle(Color.white.opacity(0.5))
                        .textCase(.uppercase)
                    
                    TextField("Name", text: $userName)
                        .textFieldStyle(.roundedBorder)
                        .focused($isNameFieldFocused)
                        .onSubmit {
                            if canGreet {
                                Task { await greet(code: selectedCode) }
                            }
                        }
                }
            }
            .frame(width: 260)
            
            Spacer()
        }
    }

    private var rightColumn: some View {
        ZStack {
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
                .frame(maxWidth: .infinity, alignment: .leading)
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
                .frame(maxWidth: .infinity, alignment: .leading)
            } else {
                let displayGreeting = greeting.isEmpty ? "Hello World!" : greeting
                VStack(spacing: 16) {
                    Text("\"\(displayGreeting)\"")
                        .font(.system(size: 52, weight: .bold))
                        .foregroundColor(Color.white.opacity(0.95))
                        .multilineTextAlignment(.center)
                        .minimumScaleFactor(0.5)
                    
                    if let selectedLanguage {
                        Text(selectedLanguage.name)
                            .font(.system(size: 16, weight: .medium))
                            .foregroundColor(Color.white.opacity(0.5))
                    }
                }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(32)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .stroke(Color.white.opacity(0.08), lineWidth: 1)
                .background(Color.white.opacity(0.02).cornerRadius(16))
        )
    }

    private var bottomActionBar: some View {
        HStack {
            Spacer()
            Button(action: {
                Task { await greet(code: selectedCode) }
            }) {
                Text(isGreeting ? "Greeting..." : "Greet")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundColor(.white)
                    .padding(.horizontal, 28)
                    .padding(.vertical, 10)
                    .background(canGreet ? Color(red: 0.0, green: 0.48, blue: 1.0) : Color.white.opacity(0.1))
                    .cornerRadius(6)
            }
            .buttonStyle(.plain)
            .disabled(!canGreet)
        }
        .padding(.horizontal, 32)
        .padding(.bottom, 32)
    }
}

/// A language returned by the daemon.
struct Language: Identifiable {
    let code: String
    let name: String
    let native: String
    var id: String { code }
}
