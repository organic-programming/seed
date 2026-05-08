import GreetingKit
import HolonsApp
import SwiftUI

struct ContentView: View {
  @ObservedObject var holonManager: GreetingHolonManager
  @ObservedObject var coaxManager: CoaxManager
  @ObservedObject var observabilityKit: ObservabilityKit
  private let initialNameValue = "World"
  private let defaultNamePrompt = "Mary"
  private let inputColumnWidth: CGFloat = 300
  private let contentSpacing: CGFloat = 32
  private let languagePickerWidth: CGFloat = 220
  private let holonSlugWidth: CGFloat = 360
  @State private var error: String?
  @State private var isLoading = true
  @State private var isGreeting = false
  @State private var didSeedInitialName = false
  @State private var isShowingCoaxSettings = false
  @State private var isShowingObservability = false
  @FocusState private var isNameFieldFocused: Bool

  private var holonSelection: Binding<GabrielHolonIdentity?> {
    Binding(
      get: { holonManager.selectedHolon },
      set: { newValue in
        guard let identity = newValue else { return }
        Task { await selectHolon(identity) }
      }
    )
  }

  private var transportSelection: Binding<String> {
    Binding(
      get: { HolonTransportName.normalize(holonManager.transport).rawValue },
      set: { newValue in
        Task { await selectTransport(named: newValue) }
      }
    )
  }

  private var languageSelection: Binding<String> {
    Binding(
      get: { holonManager.selectedLanguageCode },
      set: { newValue in
        Task { await selectLanguage(code: newValue) }
      }
    )
  }

  private var statusTitle: String {
    if isLoading {
      return "Starting holon..."
    }
    if holonManager.isRunning {
      return "Ready"
    }
    return "Offline"
  }

  private var statusColor: Color {
    if isLoading {
      return Color.orange
    }
    if holonManager.isRunning {
      return Color.green
    }
    return Color.red
  }

  var body: some View {
    VStack(spacing: 0) {
      topHeaderArea
      VStack(spacing: 24) {
        workspaceTopBar

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
    .sheet(isPresented: $isShowingCoaxSettings) {
      CoaxSettingsView(coaxManager: coaxManager, isPresented: $isShowingCoaxSettings)
    }
    .sheet(isPresented: $isShowingObservability) {
      ObservabilityPanel(kit: observabilityKit) {
        isShowingObservability = false
      }
        .frame(minWidth: 900, minHeight: 640)
    }
    .task {
      if !didSeedInitialName {
        didSeedInitialName = true
        if holonManager.userName.isEmpty {
          holonManager.userName = initialNameValue
        }
      }
      let normalized = HolonTransportName.normalize(holonManager.transport).rawValue
      if holonManager.transport != normalized {
        holonManager.transport = normalized
      }
      await loadLanguages()
    }
  }

  private func loadLanguages() async {
    isLoading = true
    error = nil
    do {
      try await holonManager.reloadLanguages(greetAfterLoad: true)
    } catch {
      self.error = "Failed to load languages: \(message(for: error))"
    }
    isLoading = false
  }

  private func selectHolon(_ identity: GabrielHolonIdentity) async {
    guard holonManager.selectedHolon != identity else { return }
    isLoading = true
    error = nil
    do {
      try await holonManager.selectHolon(slug: identity.slug, greetAfterLoad: true)
    } catch {
      self.error = "Failed to load languages: \(message(for: error))"
    }
    isLoading = false
  }

  private func selectTransport(named value: String) async {
    let normalized = HolonTransportName.normalize(holonManager.transport).rawValue
    guard value != normalized else { return }
    isLoading = true
    error = nil
    do {
      try await holonManager.selectTransport(value, greetAfterLoad: true)
    } catch {
      self.error = "Failed to load languages: \(message(for: error))"
    }
    isLoading = false
  }

  private func selectLanguage(code: String) async {
    guard code != holonManager.selectedLanguageCode else { return }
    error = nil
    do {
      try await holonManager.selectLanguageAndGreet(code)
    } catch {
      self.error = "Greeting failed: \(message(for: error))"
    }
  }

  private func greet() async {
    guard !holonManager.selectedLanguageCode.isEmpty else { return }
    isGreeting = true
    do {
      try await holonManager.greetCurrentSelection(name: holonManager.userName)
      error = nil
    } catch {
      self.error = "Greeting failed: \(message(for: error))"
    }
    isGreeting = false
  }

  private func message(for error: Error) -> String {
    if let localized = error as? LocalizedError,
      let description = localized.errorDescription,
      !description.isEmpty
    {
      return description
    }
    return error.localizedDescription
  }

  private var topHeaderArea: some View {
    HStack(alignment: .top) {
      Button {
        isShowingObservability = true
      } label: {
        Image(systemName: "waveform.path.ecg")
          .font(.system(size: 15, weight: .medium))
      }
      .buttonStyle(.borderless)
      .help("Observability")

      Spacer(minLength: 0)

      coaxHeaderGroup
    }
    .frame(maxWidth: .infinity, alignment: .center)
    .padding(.horizontal, 32)
    .padding(.vertical, 16)
    .background(Color(nsColor: .controlBackgroundColor))
    .overlay(
      Rectangle().frame(height: 1).foregroundColor(Color.primary.opacity(0.06)),
      alignment: .bottom
    )
  }

  private var workspaceTopBar: some View {
    HStack(alignment: .bottom) {
      holonHeaderGroup
      Spacer()
      runtimeHeaderGroup
    }
    .frame(maxWidth: .infinity, alignment: .center)
  }

  private var holonHeaderGroup: some View {
    VStack(alignment: .leading, spacing: 6) {
      Picker("", selection: holonSelection) {
        if holonManager.availableHolons.isEmpty {
          Text("Loading holons...").tag(nil as GabrielHolonIdentity?)
        } else {
          ForEach(holonManager.availableHolons) { identity in
            Text(identity.displayName).tag(identity as GabrielHolonIdentity?)
          }
        }
      }
      .labelsHidden()
      .frame(width: 250)

      if let slug = holonManager.selectedHolon?.slug {
        Text(slug)
          .font(.system(size: 11, weight: .regular, design: .monospaced))
          .foregroundStyle(.secondary)
          .textSelection(.enabled)
          .lineLimit(2)
          .frame(width: holonSlugWidth, alignment: .leading)
      }
    }
  }

  private var runtimeHeaderGroup: some View {
    VStack(alignment: .trailing, spacing: 8) {
      HStack(spacing: 8) {
        Text("mode:")
          .font(.system(size: 14, weight: .medium))
          .foregroundStyle(.primary)

        Picker("", selection: transportSelection) {
          ForEach(HolonTransportName.allCases) { transport in
            Text(transport.rawValue).tag(transport.rawValue)
          }
        }
        .labelsHidden()
        .frame(width: 140)
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

  private var coaxHeaderGroup: some View {
    CoaxControlsView(
      coaxManager: coaxManager,
      isShowingSettings: $isShowingCoaxSettings
    )
  }

  private var inputColumn: some View {
    VStack(alignment: .leading, spacing: 5) {
      if #available(macOS 13.0, *) {
        TextField(
          "", text: $holonManager.userName, prompt: Text(defaultNamePrompt), axis: .vertical
        )
        .lineLimit(4, reservesSpace: true)
        .textFieldStyle(.roundedBorder)
        .focused($isNameFieldFocused)
        .onChange(of: holonManager.userName) {
          Task { await greet() }
        }
        .frame(width: inputColumnWidth)
      } else {
        ZStack(alignment: .topLeading) {
          if holonManager.userName.isEmpty {
            Text(defaultNamePrompt)
              .foregroundStyle(.secondary)
              .padding(.top, 8)
              .padding(.leading, 5)
          }

          TextEditor(text: $holonManager.userName)
            .focused($isNameFieldFocused)
            .onChange(of: holonManager.userName) {
              Task { await greet() }
            }
        }
        .frame(width: inputColumnWidth, height: 100)
        .cornerRadius(6)
      }
    }
  }

  private var languagePicker: some View {
    Picker("", selection: languageSelection) {
      if isLoading {
        Text("Loading...").tag("")
      } else {
        ForEach(holonManager.availableLanguages) { language in
          Text("\(language.native) (\(language.name))").tag(language.code)
        }
      }
    }
    .labelsHidden()
    .frame(width: languagePickerWidth)
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

        if let connectionError = holonManager.connectionError {
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
          Text(holonManager.greeting)
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
