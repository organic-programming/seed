import SwiftUI

struct ContentView: View {
    @ObservedObject var daemon: DaemonProcess
    @State private var languages: [Language] = []
    @State private var selectedCode: String = ""
    @State private var userName: String = "World"
    @State private var greeting: String = ""
    @State private var error: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            Text("Gudule Greeting")
                .font(.largeTitle.bold())

            VStack(alignment: .leading, spacing: 8) {
                Text("Language")
                    .font(.headline)
                Picker("Language", selection: $selectedCode) {
                    ForEach(languages) { language in
                        Text("\(language.native) (\(language.name))")
                            .tag(language.code)
                    }
                }
                .pickerStyle(.menu)
                .labelsHidden()
                .accessibilityIdentifier("language-picker")
            }

            TextField("Your name", text: $userName)
                .textFieldStyle(.roundedBorder)
                .accessibilityIdentifier("name-input")

            Button("Greet") {
                Task { await greet(code: selectedCode) }
            }
            .disabled(selectedCode.isEmpty)
            .accessibilityIdentifier("greet-button")

            Group {
                if let error {
                    Label(error, systemImage: "exclamationmark.triangle")
                        .foregroundStyle(.red)
                } else if greeting.isEmpty {
                    Text("The greeting will appear here.")
                        .foregroundStyle(.secondary)
                } else {
                    Text(greeting)
                        .font(.largeTitle)
                        .multilineTextAlignment(.center)
                }
            }
            .frame(maxWidth: .infinity, minHeight: 180)
            .padding()
            .background {
                RoundedRectangle(cornerRadius: 18, style: .continuous)
                    .fill(Color.secondary.opacity(0.1))
            }
            .accessibilityIdentifier("greeting-output")

            Spacer(minLength: 0)
        }
        .frame(minWidth: 480, minHeight: 360, alignment: .topLeading)
        .padding(24)
        .task { await loadLanguages() }
    }

    private func loadLanguages() async {
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
                return
            } catch {
                if attempt == 2 {
                    let detail = daemon.connectionError ?? error.localizedDescription
                    self.error = "Failed to load languages: \(detail)"
                }
            }
        }
    }

    private func greet(code: String) async {
        guard !code.isEmpty else { return }
        do {
            greeting = try await daemon.sayHello(name: userName, langCode: code)
            error = nil
        } catch {
            self.error = "Greeting failed: \(error.localizedDescription)"
        }
    }
}

/// A language returned by the daemon.
struct Language: Identifiable {
    let code: String
    let name: String
    let native: String
    var id: String { code }
}
