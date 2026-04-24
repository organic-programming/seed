import SwiftUI
import Holons

public struct ObservabilityPanel: View {
    @ObservedObject private var kit: ObservabilityKit
    @ObservedObject private var gate: RuntimeGate
    @State private var tab: PanelTab = .logs
    @State private var exportStatus = ""

    public init(kit: ObservabilityKit) {
        self.kit = kit
        self.gate = kit.gate
    }

    public var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 14) {
                Picker("", selection: $tab) {
                    ForEach(PanelTab.allCases) { tab in
                        Label(tab.title, systemImage: tab.icon).tag(tab)
                    }
                }
                .labelsHidden()
                .pickerStyle(.segmented)
                .frame(maxWidth: 520)

                Spacer(minLength: 12)

                Toggle("Master", isOn: Binding(
                    get: { gate.masterEnabled },
                    set: { gate.setMaster($0) }
                ))
                .toggleStyle(.switch)

                exportButton
            }
            .padding(.horizontal, 18)
            .padding(.vertical, 12)

            Divider()

            Group {
                switch tab {
                case .logs:
                    LogConsoleView(controller: kit.logs)
                case .metrics:
                    MetricsView(controller: kit.metrics)
                case .events:
                    EventsView(controller: kit.events)
                case .settings:
                    RelaySettingsView(kit: kit)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)

            if !exportStatus.isEmpty {
                Divider()
                Text(exportStatus)
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, 18)
                    .padding(.vertical, 8)
            }
        }
        .frame(minWidth: 760, minHeight: 520)
    }

    @ViewBuilder
    private var exportButton: some View {
        #if os(macOS)
        Button {
            let panel = NSSavePanel()
            panel.canCreateDirectories = true
            panel.nameFieldStringValue = "observability-\(kit.slug)"
            panel.title = "Export Observability Snapshot"
            if panel.runModal() == .OK, let url = panel.url {
                do {
                    let out = try kit.export.exportBundle(to: url)
                    exportStatus = "Exported \(out.lastPathComponent)"
                } catch {
                    exportStatus = "Export failed: \(error.localizedDescription)"
                }
            }
        } label: {
            Label("Export", systemImage: "square.and.arrow.down")
        }
        #else
        Button {
            do {
                let out = try kit.export.export(to: FileManager.default.temporaryDirectory)
                exportStatus = "Exported \(out.lastPathComponent)"
            } catch {
                exportStatus = "Export failed: \(error.localizedDescription)"
            }
        } label: {
            Label("Export", systemImage: "square.and.arrow.down")
        }
        #endif
    }
}

private enum PanelTab: String, CaseIterable, Identifiable {
    case logs
    case metrics
    case events
    case settings

    var id: String { rawValue }

    var title: String {
        switch self {
        case .logs: return "Logs"
        case .metrics: return "Metrics"
        case .events: return "Events"
        case .settings: return "Relay"
        }
    }

    var icon: String {
        switch self {
        case .logs: return "text.alignleft"
        case .metrics: return "chart.line.uptrend.xyaxis"
        case .events: return "bolt.horizontal"
        case .settings: return "switch.2"
        }
    }
}

public struct RelaySettingsView: View {
    @ObservedObject private var kit: ObservabilityKit
    @ObservedObject private var gate: RuntimeGate
    @ObservedObject private var prometheus: PrometheusController

    public init(kit: ObservabilityKit) {
        self.kit = kit
        self.gate = kit.gate
        self.prometheus = kit.prometheus
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                settingsSection("Runtime Gates") {
                    gateToggle("Master", isOn: Binding(
                        get: { gate.masterEnabled },
                        set: { value in
                            gate.setMaster(value)
                            prometheus.sync()
                        }
                    ))
                    gateToggle("Logs", isOn: familyBinding(.logs))
                    gateToggle("Metrics", isOn: familyBinding(.metrics))
                    gateToggle("Events", isOn: familyBinding(.events))
                    gateToggle("Prometheus", isOn: Binding(
                        get: { gate.promEnabled },
                        set: { value in
                            gate.setFamily(.prom, value)
                            if value {
                                try? prometheus.start()
                            } else {
                                prometheus.stop()
                            }
                        }
                    ))

                    if !prometheus.boundAddress.isEmpty || !gate.promAddress.isEmpty {
                        Text(prometheus.boundAddress.isEmpty ? gate.promAddress : prometheus.boundAddress)
                            .font(.system(.caption, design: .monospaced))
                            .textSelection(.enabled)
                            .foregroundStyle(.secondary)
                    }
                }

                settingsSection("Bundled Members") {
                    if gate.members.isEmpty {
                        Text("No bundled members declared.")
                            .font(.callout)
                            .foregroundStyle(.secondary)
                    } else {
                        ForEach(gate.members) { member in
                            HStack(spacing: 12) {
                                VStack(alignment: .leading, spacing: 3) {
                                    Text(member.slug)
                                        .font(.headline)
                                    Text(member.uid)
                                        .font(.system(.caption, design: .monospaced))
                                        .foregroundStyle(.secondary)
                                    if !member.address.isEmpty {
                                        Text(member.address)
                                            .font(.system(.caption, design: .monospaced))
                                            .foregroundStyle(.secondary)
                                    }
                                }

                                Spacer(minLength: 12)

                                Picker("", selection: Binding(
                                    get: { gate.memberOverride(member.uid) },
                                    set: { gate.setMemberOverride(member.uid, $0) }
                                )) {
                                    Text("Default").tag(GateOverride.defaultValue)
                                    Text("On").tag(GateOverride.on)
                                    Text("Off").tag(GateOverride.off)
                                }
                                .labelsHidden()
                                .pickerStyle(.segmented)
                                .frame(width: 240)
                            }
                            .padding(.vertical, 8)
                        }
                    }
                }
            }
            .padding(18)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
    }

    private func familyBinding(_ family: Family) -> Binding<Bool> {
        Binding(
            get: { gate.familyEnabled(family) },
            set: { gate.setFamily(family, $0) }
        )
    }
}

public struct LogConsoleView: View {
    @ObservedObject private var controller: ConsoleController

    public init(controller: ConsoleController) {
        self.controller = controller
    }

    public var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 12) {
                Picker("Level", selection: $controller.minLevel) {
                    ForEach([Level.trace, .debug, .info, .warn, .error, .fatal], id: \.self) { level in
                        Text(level.name).tag(level)
                    }
                }
                .pickerStyle(.menu)
                .frame(width: 160)

                TextField("Filter logs", text: $controller.query)
                    .textFieldStyle(.roundedBorder)
            }
            .padding(14)

            Divider()

            List(controller.filteredEntries.indices, id: \.self) { index in
                let entry = controller.filteredEntries[index]
                VStack(alignment: .leading, spacing: 6) {
                    HStack(spacing: 8) {
                        Text(entry.level.name)
                            .font(.system(.caption, design: .monospaced).weight(.bold))
                            .foregroundStyle(levelColor(entry.level))
                        Text(entry.slug)
                            .font(.system(.caption, design: .monospaced))
                            .foregroundStyle(.secondary)
                        Text(entry.timestamp.formatted(date: .omitted, time: .standard))
                            .font(.caption)
                            .foregroundStyle(.tertiary)
                        if !entry.chain.isEmpty {
                            Text(chainText(entry.chain))
                                .font(.system(.caption2, design: .monospaced))
                                .foregroundStyle(.secondary)
                        }
                    }

                    Text(entry.message)
                        .font(.body)
                        .textSelection(.enabled)

                    if !entry.fields.isEmpty {
                        Text(entry.fields.keys.sorted().map { "\($0)=\(entry.fields[$0] ?? "")" }.joined(separator: " "))
                            .font(.system(.caption, design: .monospaced))
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                    }
                }
                .padding(.vertical, 6)
            }
            .listStyle(.plain)
        }
    }
}

public struct MetricsView: View {
    @ObservedObject private var controller: MetricsController

    public init(controller: MetricsController) {
        self.controller = controller
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                if let latest = controller.latest {
                    metricSection("Counters") {
                        metricRows(latest.counters.map { ($0.name, String($0.read()), $0.labels) })
                    }
                    metricSection("Gauges") {
                        ForEach(latest.gauges, id: \.name) { gauge in
                            VStack(alignment: .leading, spacing: 7) {
                                HStack {
                                    metricName(gauge.name, labels: gauge.labels)
                                    Spacer()
                                    Text(String(format: "%.3f", gauge.read()))
                                        .font(.system(.callout, design: .monospaced))
                                }
                                SparklineView(values: gaugeHistory(named: gauge.name, labels: gauge.labels))
                                    .frame(height: 34)
                            }
                        }
                    }
                    metricSection("Histograms") {
                        ForEach(latest.histograms, id: \.name) { histogram in
                            VStack(alignment: .leading, spacing: 9) {
                                metricName(histogram.name, labels: histogram.labels)
                                HistogramChart(snapshot: histogram.snapshot())
                                    .frame(height: 86)
                            }
                        }
                    }
                } else {
                    Text("No metric registry is active.")
                        .foregroundStyle(.secondary)
                }
            }
            .padding(18)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
    }

    private func gaugeHistory(named name: String, labels: [String: String]) -> [Double] {
        controller.history.compactMap { snapshot in
            snapshot.gauges.first { $0.name == name && $0.labels == labels }?.read()
        }
    }
}

public struct EventsView: View {
    @ObservedObject private var controller: EventsController

    public init(controller: EventsController) {
        self.controller = controller
    }

    public var body: some View {
        List(controller.events.indices, id: \.self) { index in
            let event = controller.events[index]
            VStack(alignment: .leading, spacing: 6) {
                HStack(spacing: 8) {
                    Text(event.type.protoName)
                        .font(.system(.caption, design: .monospaced).weight(.bold))
                        .foregroundStyle(.blue)
                    Text(event.slug)
                        .font(.system(.caption, design: .monospaced))
                        .foregroundStyle(.secondary)
                    Text(event.timestamp.formatted(date: .omitted, time: .standard))
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                    if !event.chain.isEmpty {
                        Text(chainText(event.chain))
                            .font(.system(.caption2, design: .monospaced))
                            .foregroundStyle(.secondary)
                    }
                }

                if !event.payload.isEmpty {
                    Text(event.payload.keys.sorted().map { "\($0)=\(event.payload[$0] ?? "")" }.joined(separator: " "))
                        .font(.system(.caption, design: .monospaced))
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                }
            }
            .padding(.vertical, 6)
        }
        .listStyle(.plain)
    }
}

public struct SparklineView: View {
    public let values: [Double]

    public init(values: [Double]) {
        self.values = values
    }

    public var body: some View {
        GeometryReader { geo in
            let points = sparkPoints(size: geo.size)
            Path { path in
                guard let first = points.first else { return }
                path.move(to: first)
                for point in points.dropFirst() {
                    path.addLine(to: point)
                }
            }
            .stroke(Color.accentColor, style: StrokeStyle(lineWidth: 2, lineJoin: .round))
            .background(Color.secondary.opacity(0.08))
            .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        }
        .accessibilityLabel("Metric sparkline")
    }

    private func sparkPoints(size: CGSize) -> [CGPoint] {
        guard values.count > 1 else { return [] }
        let minValue = values.min() ?? 0
        let maxValue = values.max() ?? 0
        let span = max(maxValue - minValue, 0.000_001)
        return values.enumerated().map { offset, value in
            let x = size.width * CGFloat(offset) / CGFloat(values.count - 1)
            let y = size.height - size.height * CGFloat((value - minValue) / span)
            return CGPoint(x: x, y: y)
        }
    }
}

public struct HistogramChart: View {
    public let snapshot: HistogramSnapshot

    public init(snapshot: HistogramSnapshot) {
        self.snapshot = snapshot
    }

    public var body: some View {
        GeometryReader { geo in
            let maxCount = max(snapshot.counts.max() ?? 0, 1)
            HStack(alignment: .bottom, spacing: 3) {
                ForEach(snapshot.counts.indices, id: \.self) { index in
                    RoundedRectangle(cornerRadius: 2, style: .continuous)
                        .fill(Color.accentColor.opacity(0.72))
                        .frame(height: max(2, geo.size.height * CGFloat(snapshot.counts[index]) / CGFloat(maxCount)))
                        .help("<= \(snapshot.bounds[index])")
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottom)
            .background(Color.secondary.opacity(0.08))
            .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        }
        .accessibilityLabel("Histogram chart")
    }
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
        .padding(.top, 4)
    } label: {
        Text(title)
            .font(.headline)
    }
}

@MainActor
private func gateToggle(_ title: String, isOn: Binding<Bool>) -> some View {
    Toggle(title, isOn: isOn)
        .toggleStyle(.switch)
}

private func metricSection<Content: View>(
    _ title: String,
    @ViewBuilder content: () -> Content
) -> some View {
    VStack(alignment: .leading, spacing: 10) {
        Text(title)
            .font(.headline)
        content()
    }
}

private func metricRows(_ rows: [(String, String, [String: String])]) -> some View {
    ForEach(rows, id: \.0) { name, value, labels in
        HStack {
            metricName(name, labels: labels)
            Spacer()
            Text(value)
                .font(.system(.callout, design: .monospaced))
        }
    }
}

private func metricName(_ name: String, labels: [String: String]) -> some View {
    VStack(alignment: .leading, spacing: 3) {
        Text(name)
            .font(.system(.body, design: .monospaced))
        if !labels.isEmpty {
            Text(labels.keys.sorted().map { "\($0)=\(labels[$0] ?? "")" }.joined(separator: ", "))
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }
}

private func levelColor(_ level: Level) -> Color {
    switch level {
    case .trace, .debug: return .secondary
    case .info: return .blue
    case .warn: return .orange
    case .error, .fatal: return .red
    case .unset: return .secondary
    }
}

private func chainText(_ chain: [Hop]) -> String {
    chain.map { "\($0.slug):\($0.instanceUid)" }.joined(separator: " > ")
}
