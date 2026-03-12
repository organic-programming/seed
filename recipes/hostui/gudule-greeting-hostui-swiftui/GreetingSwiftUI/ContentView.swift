import SwiftUI

struct ContentView: View {
    @ObservedObject var daemon: DaemonProcess
    private let assemblyFamily = ProcessInfo.processInfo.environment["OP_ASSEMBLY_DISPLAY_FAMILY"]
        ?? ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]
        ?? "Greeting-Swiftui-Go (SwiftUI)"
    @State private var languages: [Language] = []
    @State private var selectedCode: String = ""
    @State private var userName: String = ""
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
            return "Starting daemon"
        }
        if daemon.isRunning {
            return "Connected"
        }
        return "Offline"
    }

    private var statusColor: Color {
        if isLoading {
            return Color.orange.opacity(0.9)
        }
        if daemon.isRunning {
            return Color.green.opacity(0.9)
        }
        return Color.red.opacity(0.85)
    }

    var body: some View {
        GeometryReader { proxy in
            ZStack {
                backgroundLayer

                ScrollView {
                    VStack(spacing: 24) {
                        heroCard
                        contentColumns(for: proxy.size.width)
                    }
                    .frame(maxWidth: proxy.size.width >= 1080 ? 1260 : 960)
                    .padding(.horizontal, 32)
                    .padding(.vertical, 36)
                }
            }
        }
        .frame(minWidth: 1080, minHeight: 720, alignment: .topLeading)
        .animation(.spring(response: 0.35, dampingFraction: 0.88), value: greeting)
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
                if trimmedUserName.isEmpty {
                    userName = ""
                    isNameFieldFocused = true
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

    private var backgroundLayer: some View {
        ZStack {
            LinearGradient(
                colors: [
                    Color(red: 0.08, green: 0.09, blue: 0.14),
                    Color(red: 0.12, green: 0.13, blue: 0.18),
                    Color(red: 0.05, green: 0.06, blue: 0.1),
                ],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .ignoresSafeArea()

            Circle()
                .fill(Color(red: 0.22, green: 0.44, blue: 0.84).opacity(0.25))
                .frame(width: 320, height: 320)
                .blur(radius: 90)
                .offset(x: -260, y: -180)

            Circle()
                .fill(Color(red: 0.17, green: 0.72, blue: 0.63).opacity(0.18))
                .frame(width: 280, height: 280)
                .blur(radius: 90)
                .offset(x: 340, y: 220)
        }
    }

    private var heroCard: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack(spacing: 10) {
                badge(title: statusTitle, color: statusColor)
                badge(title: "\(max(languages.count, 56)) languages", color: Color.white.opacity(0.18))
                badge(title: "SwiftUI Host", color: Color.white.opacity(0.12))
            }

            VStack(alignment: .leading, spacing: 8) {
                Text("Gudule")
                    .font(.system(size: 16, weight: .semibold, design: .rounded))
                    .foregroundStyle(Color.white.opacity(0.72))
                    .textCase(.uppercase)

                Text(assemblyFamily)
                    .font(.system(size: 44, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                    .lineLimit(2)
                    .minimumScaleFactor(0.72)

                Text("A native macOS greeting host with a bundled daemon discovered through connect(slug).")
                    .font(.system(size: 16, weight: .medium, design: .rounded))
                    .foregroundStyle(Color.white.opacity(0.68))
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(28)
        .background(cardBackground)
    }

    private var controlCard: some View {
        VStack(alignment: .leading, spacing: 22) {
            VStack(alignment: .leading, spacing: 6) {
                Text("Compose a Greeting")
                    .font(.system(size: 28, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                Text("Pick a language, enter a name, and let the daemon respond.")
                    .font(.system(size: 15, weight: .medium, design: .rounded))
                    .foregroundStyle(Color.white.opacity(0.66))
            }

            VStack(alignment: .leading, spacing: 10) {
                sectionLabel("Language")
                languageMenu
            }

            VStack(alignment: .leading, spacing: 10) {
                sectionLabel("Name")
                TextField("Enter your name", text: $userName)
                    .textFieldStyle(.plain)
                    .font(.system(size: 20, weight: .semibold, design: .rounded))
                    .foregroundStyle(.white)
                    .padding(.horizontal, 18)
                    .padding(.vertical, 16)
                    .background(inputBackground(strokeColor: isNameFieldFocused ? Color.white.opacity(0.5) : Color.white.opacity(0.18)))
                    .overlay(alignment: .trailing) {
                        if isGreeting {
                            ProgressView()
                                .tint(.white.opacity(0.85))
                                .padding(.trailing, 18)
                        }
                    }
                    .focused($isNameFieldFocused)
                    .accessibilityIdentifier("name-input")
                    .onSubmit {
                        if canGreet {
                            Task { await greet(code: selectedCode) }
                        }
                    }
            }

            Button {
                Task { await greet(code: selectedCode) }
            } label: {
                HStack(spacing: 12) {
                    Image(systemName: isGreeting ? "hourglass" : "sparkles")
                        .font(.system(size: 15, weight: .bold))
                    Text(isGreeting ? "Greeting..." : "Greet")
                        .font(.system(size: 18, weight: .bold, design: .rounded))
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical, 16)
                .foregroundStyle(.white)
                .background(
                    LinearGradient(
                        colors: [
                            Color(red: 0.22, green: 0.53, blue: 0.95),
                            Color(red: 0.12, green: 0.74, blue: 0.62),
                        ],
                        startPoint: .leading,
                        endPoint: .trailing
                    )
                )
                .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
                .shadow(color: Color.black.opacity(0.25), radius: 18, y: 10)
            }
            .buttonStyle(.plain)
            .disabled(!canGreet)
            .opacity(canGreet ? 1.0 : 0.55)
            .accessibilityIdentifier("greet-button")
        }
        .padding(28)
        .frame(maxWidth: .infinity, alignment: .topLeading)
        .background(cardBackground)
    }

    private var resultCard: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Result")
                        .font(.system(size: 28, weight: .bold, design: .rounded))
                        .foregroundStyle(.white)
                    Text(resultCaption)
                        .font(.system(size: 15, weight: .medium, design: .rounded))
                        .foregroundStyle(Color.white.opacity(0.66))
                }

                Spacer()

                if let selectedLanguage {
                    badge(title: selectedLanguage.native, color: Color.white.opacity(0.12))
                }
            }

            Group {
                if let error {
                    HStack(alignment: .top, spacing: 14) {
                        Image(systemName: "exclamationmark.triangle.fill")
                            .foregroundStyle(Color(red: 1.0, green: 0.48, blue: 0.42))
                            .font(.system(size: 18, weight: .bold))
                        Text(error)
                            .font(.system(size: 18, weight: .semibold, design: .rounded))
                            .foregroundStyle(Color.white.opacity(0.92))
                            .fixedSize(horizontal: false, vertical: true)
                    }
                    .frame(maxWidth: .infinity, minHeight: 220, alignment: .leading)
                    .padding(26)
                    .background(
                        RoundedRectangle(cornerRadius: 24, style: .continuous)
                            .fill(Color(red: 0.32, green: 0.08, blue: 0.08).opacity(0.72))
                    )
                } else if greeting.isEmpty {
                    VStack(spacing: 14) {
                        Image(systemName: "text.bubble")
                            .font(.system(size: 28, weight: .medium))
                            .foregroundStyle(Color.white.opacity(0.8))
                        Text("Your localized greeting will land here.")
                            .font(.system(size: 24, weight: .bold, design: .rounded))
                            .foregroundStyle(.white)
                        Text("Use the controls above to send a SayHello request.")
                            .font(.system(size: 16, weight: .medium, design: .rounded))
                            .foregroundStyle(Color.white.opacity(0.62))
                    }
                    .frame(maxWidth: .infinity, minHeight: 240)
                    .background(resultSurface)
                } else {
                    VStack(spacing: 14) {
                        Image(systemName: "quote.opening")
                            .font(.system(size: 22, weight: .bold))
                            .foregroundStyle(Color.white.opacity(0.55))
                        Text(greeting)
                            .font(.system(size: 44, weight: .bold, design: .rounded))
                            .foregroundStyle(.white)
                            .multilineTextAlignment(.center)
                            .minimumScaleFactor(0.7)
                        if let selectedLanguage {
                            Text("Delivered in \(selectedLanguage.name)")
                                .font(.system(size: 15, weight: .semibold, design: .rounded))
                                .foregroundStyle(Color.white.opacity(0.58))
                        }
                    }
                    .frame(maxWidth: .infinity, minHeight: 240)
                    .padding(24)
                    .background(resultSurface)
                }
            }
            .accessibilityIdentifier("greeting-output")
        }
        .padding(28)
        .frame(maxWidth: .infinity, minHeight: 432, alignment: .topLeading)
        .background(cardBackground)
    }

    @ViewBuilder
    private func contentColumns(for width: CGFloat) -> some View {
        if width >= 1080 {
            HStack(alignment: .top, spacing: 24) {
                controlCard
                    .frame(maxWidth: 560, alignment: .topLeading)
                resultCard
                    .frame(maxWidth: .infinity, alignment: .topLeading)
            }
        } else {
            VStack(spacing: 24) {
                controlCard
                resultCard
            }
        }
    }

    private var languageMenu: some View {
        Menu {
            ForEach(languages) { language in
                Button {
                    selectedCode = language.code
                } label: {
                    if selectedCode == language.code {
                        Label("\(language.native) (\(language.name))", systemImage: "checkmark")
                    } else {
                        Text("\(language.native) (\(language.name))")
                    }
                }
            }
        } label: {
            HStack(spacing: 14) {
                Image(systemName: "globe.europe.africa.fill")
                    .foregroundStyle(Color.white.opacity(0.7))
                    .font(.system(size: 18, weight: .semibold))

                VStack(alignment: .leading, spacing: 2) {
                    Text(selectedLanguage?.native ?? (isLoading ? "Loading languages..." : "Select a language"))
                        .font(.system(size: 20, weight: .semibold, design: .rounded))
                        .foregroundStyle(.white)
                    Text(selectedLanguage?.name ?? "Greeting language")
                        .font(.system(size: 13, weight: .medium, design: .rounded))
                        .foregroundStyle(Color.white.opacity(0.56))
                }

                Spacer()

                Image(systemName: "chevron.up.chevron.down")
                    .foregroundStyle(Color.white.opacity(0.45))
                    .font(.system(size: 14, weight: .bold))
            }
            .padding(.horizontal, 18)
            .padding(.vertical, 16)
            .background(inputBackground(strokeColor: Color.white.opacity(0.18)))
        }
        .buttonStyle(.plain)
        .disabled(isLoading || languages.isEmpty)
        .accessibilityIdentifier("language-picker")
    }

    private func badge(title: String, color: Color) -> some View {
        Text(title)
            .font(.system(size: 12, weight: .bold, design: .rounded))
            .foregroundStyle(.white)
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .background(
                Capsule(style: .continuous)
                    .fill(color)
            )
    }

    private func sectionLabel(_ title: String) -> some View {
        Text(title)
            .font(.system(size: 13, weight: .bold, design: .rounded))
            .foregroundStyle(Color.white.opacity(0.74))
            .textCase(.uppercase)
            .tracking(0.8)
    }

    private func inputBackground(strokeColor: Color) -> some View {
        RoundedRectangle(cornerRadius: 18, style: .continuous)
            .fill(Color.white.opacity(0.07))
            .overlay(
                RoundedRectangle(cornerRadius: 18, style: .continuous)
                    .stroke(strokeColor, lineWidth: 1.5)
            )
    }

    private var resultSurface: some View {
        RoundedRectangle(cornerRadius: 24, style: .continuous)
            .fill(
                LinearGradient(
                    colors: [
                        Color.white.opacity(0.08),
                        Color.white.opacity(0.04),
                    ],
                    startPoint: .topLeading,
                    endPoint: .bottomTrailing
                )
            )
            .overlay(
                RoundedRectangle(cornerRadius: 24, style: .continuous)
                    .stroke(Color.white.opacity(0.08), lineWidth: 1.0)
            )
    }

    private var cardBackground: some View {
        RoundedRectangle(cornerRadius: 28, style: .continuous)
            .fill(Color.black.opacity(0.28))
            .overlay(
                RoundedRectangle(cornerRadius: 28, style: .continuous)
                    .stroke(Color.white.opacity(0.08), lineWidth: 1.0)
            )
            .shadow(color: Color.black.opacity(0.24), radius: 28, y: 12)
    }

    private var resultCaption: String {
        if let error {
            return error.isEmpty ? "Something went wrong." : "Something needs attention."
        }
        if greeting.isEmpty {
            return "Ready when you are."
        }
        return "The daemon replied successfully."
    }
}

/// A language returned by the daemon.
struct Language: Identifiable {
    let code: String
    let name: String
    let native: String
    var id: String { code }
}
