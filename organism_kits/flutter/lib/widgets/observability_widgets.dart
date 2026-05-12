import 'dart:io';

import 'package:flutter/material.dart';
import 'package:holons/holons.dart' as holons;

import '../observability/observability_kit.dart';

class ObservabilityPanel extends StatefulWidget {
  const ObservabilityPanel({
    super.key,
    required this.kit,
    this.exportDestination,
  });

  final ObservabilityKit kit;
  final Future<Directory?> Function()? exportDestination;

  @override
  State<ObservabilityPanel> createState() => _ObservabilityPanelState();
}

class _ObservabilityPanelState extends State<ObservabilityPanel>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs;
  bool _exporting = false;
  String _exportStatus = '';

  @override
  void initState() {
    super.initState();
    _tabs = TabController(length: 4, vsync: this);
  }

  @override
  void dispose() {
    _tabs.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: widget.kit.gate,
      builder: (context, _) {
        return Column(
          children: [
            Material(
              color: Theme.of(context).colorScheme.surface,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: TabBar(
                          controller: _tabs,
                          tabs: const [
                            Tab(icon: Icon(Icons.tune), text: 'Settings'),
                            Tab(icon: Icon(Icons.notes), text: 'Logs'),
                            Tab(
                              icon: Icon(Icons.monitor_heart),
                              text: 'Metrics',
                            ),
                            Tab(icon: Icon(Icons.event_note), text: 'Events'),
                          ],
                        ),
                      ),
                      IconButton(
                        tooltip: 'Export observability bundle',
                        icon: const Icon(Icons.file_download_outlined),
                        onPressed:
                            widget.exportDestination == null || _exporting
                            ? null
                            : _export,
                      ),
                    ],
                  ),
                  if (_exportStatus.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: Text(_exportStatus),
                    ),
                ],
              ),
            ),
            Expanded(
              child: TabBarView(
                controller: _tabs,
                children: [
                  RelaySettingsView(kit: widget.kit),
                  LogConsoleView(controller: widget.kit.logs),
                  MetricsView(controller: widget.kit.metrics),
                  EventsView(controller: widget.kit.events),
                ],
              ),
            ),
          ],
        );
      },
    );
  }

  Future<void> _export() async {
    final destination = widget.exportDestination;
    if (destination == null) return;
    setState(() {
      _exporting = true;
      _exportStatus = '';
    });
    try {
      final dir = await destination();
      if (dir == null) {
        if (mounted) {
          setState(() {
            _exporting = false;
          });
        }
        return;
      }
      final exported = await widget.kit.export.exportTo(dir);
      if (!mounted) return;
      setState(() {
        _exporting = false;
        _exportStatus = 'Exported to ${exported.path}';
      });
    } on Object catch (error) {
      if (!mounted) return;
      setState(() {
        _exporting = false;
        _exportStatus = 'Export failed: $error';
      });
    }
  }
}

class RelaySettingsView extends StatelessWidget {
  const RelaySettingsView({super.key, required this.kit});

  final ObservabilityKit kit;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: kit.gate,
      builder: (context, _) {
        final gate = kit.gate;
        return ListView(
          padding: const EdgeInsets.all(16),
          children: [
            SwitchListTile(
              value: gate.masterEnabled,
              title: const Text('Master'),
              secondary: const Icon(Icons.power_settings_new),
              onChanged: gate.setMaster,
            ),
            SwitchListTile(
              value: gate.logsEnabled,
              title: const Text('Logs'),
              secondary: const Icon(Icons.notes),
              onChanged: (value) => gate.setFamily(holons.Family.logs, value),
            ),
            SwitchListTile(
              value: gate.metricsEnabled,
              title: const Text('Metrics'),
              secondary: const Icon(Icons.monitor_heart),
              onChanged: (value) =>
                  gate.setFamily(holons.Family.metrics, value),
            ),
            SwitchListTile(
              value: gate.eventsEnabled,
              title: const Text('Events'),
              secondary: const Icon(Icons.event_note),
              onChanged: (value) => gate.setFamily(holons.Family.events, value),
            ),
            SwitchListTile(
              value: gate.promEnabled,
              title: const Text('Prometheus /metrics'),
              subtitle: Text(
                gate.promAddress.isEmpty ? 'Not bound' : gate.promAddress,
              ),
              secondary: const Icon(Icons.http),
              onChanged: (value) => gate.setFamily(holons.Family.prom, value),
            ),
            const Divider(height: 32),
            for (final member in gate.members)
              ListTile(
                leading: const Icon(Icons.hub),
                title: Text(member.slug),
                subtitle: Text(member.uid),
                trailing: DropdownButton<GateOverride>(
                  value: gate.memberOverride(member.uid),
                  items: const [
                    DropdownMenuItem(
                      value: GateOverride.defaultValue,
                      child: Text('Default'),
                    ),
                    DropdownMenuItem(value: GateOverride.on, child: Text('On')),
                    DropdownMenuItem(
                      value: GateOverride.off,
                      child: Text('Off'),
                    ),
                  ],
                  onChanged: (value) {
                    if (value != null)
                      gate.setMemberOverride(member.uid, value);
                  },
                ),
              ),
          ],
        );
      },
    );
  }
}

class LogConsoleView extends StatelessWidget {
  const LogConsoleView({super.key, required this.controller});

  final LogConsoleController controller;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        final entries = controller.entries;
        return Column(
          children: [
            Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  DropdownButton<holons.Level>(
                    value: controller.minLevel,
                    items: holons.Level.values
                        .where((level) => level != holons.Level.unset)
                        .map(
                          (level) => DropdownMenuItem(
                            value: level,
                            child: Text(level.name.toUpperCase()),
                          ),
                        )
                        .toList(growable: false),
                    onChanged: (value) {
                      if (value != null) controller.setMinLevel(value);
                    },
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: TextField(
                      decoration: const InputDecoration(
                        prefixIcon: Icon(Icons.search),
                        hintText: 'Filter logs',
                        isDense: true,
                        border: OutlineInputBorder(),
                      ),
                      onChanged: controller.setQuery,
                    ),
                  ),
                ],
              ),
            ),
            Expanded(
              child: ListView.builder(
                itemCount: entries.length,
                itemBuilder: (context, index) {
                  final entry = entries[entries.length - index - 1];
                  final secondaryStyle = Theme.of(context).textTheme.bodySmall;
                  final fieldsText = _fieldsText(entry.fields);
                  final contextText = _logContextText(entry);
                  return ListTile(
                    dense: true,
                    trailing: _LevelBadge(level: entry.level),
                    title: _LogTitle(entry: entry),
                    subtitle: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (contextText.isNotEmpty)
                          Text(contextText, style: secondaryStyle),
                        if (fieldsText.isNotEmpty)
                          Text(
                            fieldsText,
                            style: secondaryStyle?.copyWith(
                              fontFamily: 'monospace',
                            ),
                          ),
                        Text(_logOriginText(entry), style: secondaryStyle),
                        if (entry.chain.isNotEmpty)
                          Text(
                            '← ${_chainText(entry.chain)}',
                            style: secondaryStyle,
                          ),
                      ],
                    ),
                  );
                },
              ),
            ),
          ],
        );
      },
    );
  }
}

class _LogTitle extends StatelessWidget {
  const _LogTitle({required this.entry});

  final holons.LogEntry entry;

  @override
  Widget build(BuildContext context) {
    if (entry.loggerName.isEmpty) {
      return Text(entry.message);
    }
    final style = DefaultTextStyle.of(context).style;
    return Text.rich(
      TextSpan(
        children: [
          TextSpan(
            text: '[${entry.loggerName}]  ',
            style: style.copyWith(fontFamily: 'monospace'),
          ),
          TextSpan(text: entry.message),
        ],
      ),
    );
  }
}

class MetricsView extends StatelessWidget {
  const MetricsView({super.key, required this.controller});

  final MetricsController controller;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        final latest = controller.latest;
        if (latest == null ||
            !controller.gate.familyEnabled(holons.Family.metrics)) {
          return const Center(child: Text('No metric samples'));
        }
        return ListView(
          padding: const EdgeInsets.all(16),
          children: [
            for (final counter in latest.counters)
              ListTile(
                leading: const Icon(Icons.add_circle_outline),
                title: Text(counter.name),
                subtitle: Text(counter.help),
                trailing: Text(counter.value().toString()),
              ),
            for (final gauge in latest.gauges)
              ListTile(
                leading: SizedBox(
                  width: 72,
                  height: 24,
                  child: SparklineView(
                    values: controller.history
                        .map(
                          (snapshot) => snapshot.gauges
                              .where((g) => g.name == gauge.name)
                              .map((g) => g.value())
                              .cast<double?>()
                              .firstOrNull,
                        )
                        .whereType<double>()
                        .toList(growable: false),
                  ),
                ),
                title: Text(gauge.name),
                subtitle: Text(gauge.help),
                trailing: Text(gauge.value().toStringAsFixed(2)),
              ),
            for (final histogram in latest.histograms)
              ListTile(
                leading: SizedBox(
                  width: 96,
                  height: 28,
                  child: HistogramChart(snapshot: histogram.snapshot()),
                ),
                title: Text(histogram.name),
                subtitle: Text(histogram.help),
                trailing: Text(histogram.snapshot().total.toString()),
              ),
          ],
        );
      },
    );
  }
}

class EventsView extends StatelessWidget {
  const EventsView({super.key, required this.controller});

  final EventsController controller;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        final events = controller.events;
        return ListView.builder(
          padding: const EdgeInsets.all(12),
          itemCount: events.length,
          itemBuilder: (context, index) {
            final event = events[events.length - index - 1];
            return ListTile(
              leading: const Icon(Icons.bolt),
              title: Text(event.type.name),
              subtitle: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('${event.slug}  ${event.timestamp.toIso8601String()}'),
                  if (event.chain.isNotEmpty) Text(_chainText(event.chain)),
                ],
              ),
            );
          },
        );
      },
    );
  }
}

class SparklineView extends StatelessWidget {
  const SparklineView({super.key, required this.values});

  final List<double> values;

  @override
  Widget build(BuildContext context) {
    return CustomPaint(
      painter: _SparklinePainter(values, Theme.of(context).colorScheme.primary),
      size: const Size(double.infinity, 24),
    );
  }
}

class HistogramChart extends StatelessWidget {
  const HistogramChart({super.key, required this.snapshot});

  final holons.HistogramSnapshot snapshot;

  @override
  Widget build(BuildContext context) {
    return CustomPaint(
      painter: _HistogramPainter(
        snapshot,
        Theme.of(context).colorScheme.secondary,
      ),
      size: const Size(double.infinity, 28),
    );
  }
}

class _LevelBadge extends StatelessWidget {
  const _LevelBadge({required this.level});

  final holons.Level level;

  @override
  Widget build(BuildContext context) {
    final color = switch (level) {
      holons.Level.trace => Colors.blueGrey,
      holons.Level.debug => Colors.blue,
      holons.Level.info => Colors.green,
      holons.Level.warn => Colors.orange,
      holons.Level.error => Colors.red,
      holons.Level.fatal => Colors.purple,
      holons.Level.unset => Colors.grey,
    };
    return Chip(
      visualDensity: VisualDensity.compact,
      label: Text(level.name.toUpperCase()),
      backgroundColor: color.withValues(alpha: 0.12),
      labelStyle: TextStyle(color: color, fontWeight: FontWeight.w600),
    );
  }
}

class _SparklinePainter extends CustomPainter {
  _SparklinePainter(this.values, this.color);
  final List<double> values;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    if (values.length < 2) return;
    final min = values.reduce((a, b) => a < b ? a : b);
    final max = values.reduce((a, b) => a > b ? a : b);
    final span = (max - min).abs() < 0.000001 ? 1.0 : max - min;
    final path = Path();
    for (var i = 0; i < values.length; i++) {
      final x = size.width * i / (values.length - 1);
      final y = size.height - ((values[i] - min) / span * size.height);
      if (i == 0) {
        path.moveTo(x, y);
      } else {
        path.lineTo(x, y);
      }
    }
    canvas.drawPath(
      path,
      Paint()
        ..color = color
        ..strokeWidth = 2
        ..style = PaintingStyle.stroke,
    );
  }

  @override
  bool shouldRepaint(covariant _SparklinePainter oldDelegate) =>
      oldDelegate.values != values || oldDelegate.color != color;
}

class _HistogramPainter extends CustomPainter {
  _HistogramPainter(this.snapshot, this.color);
  final holons.HistogramSnapshot snapshot;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    if (snapshot.counts.isEmpty) return;
    final max = snapshot.counts.reduce((a, b) => a > b ? a : b);
    if (max <= 0) return;
    final width = size.width / snapshot.counts.length;
    final paint = Paint()..color = color.withValues(alpha: 0.65);
    for (var i = 0; i < snapshot.counts.length; i++) {
      final h = snapshot.counts[i] / max * size.height;
      canvas.drawRect(
        Rect.fromLTWH(i * width, size.height - h, width * 0.72, h),
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(covariant _HistogramPainter oldDelegate) =>
      oldDelegate.snapshot != snapshot || oldDelegate.color != color;
}

String _chainText(List<holons.Hop> chain) {
  return chain.map((hop) => '${hop.slug}:${hop.instanceUid}').join(' > ');
}

String _logContextText(holons.LogEntry entry) {
  final parts = <String>[];
  if (entry.rpcMethod.isNotEmpty) {
    parts.add(entry.rpcMethod);
  }
  if (entry.sessionId.isNotEmpty) {
    final session = entry.sessionId.length <= 8
        ? entry.sessionId
        : entry.sessionId.substring(0, 8);
    parts.add('session $session');
  }
  return parts.join(' · ');
}

String _logOriginText(holons.LogEntry entry) {
  final parts = <String>[
    entry.slug,
    entry.timestamp.toIso8601String(),
    if (entry.caller.isNotEmpty) entry.caller,
  ];
  return parts.join(' · ');
}

String _fieldsText(Map<String, String> fields) {
  if (fields.isEmpty) return '';
  final keys = fields.keys.toList()..sort();
  return keys
      .map((key) => '$key=${_fieldValueText(fields[key] ?? '')}')
      .join('  ');
}

String _fieldValueText(String value) {
  final needsQuotes =
      value.isEmpty || value.contains(RegExp(r'\s')) || value.contains('"');
  if (!needsQuotes) return value;
  return '"${value.replaceAll('"', r'\"')}"';
}

extension _IterableFirstOrNull<T> on Iterable<T> {
  T? get firstOrNull => isEmpty ? null : first;
}
