import 'package:flutter/material.dart';

import '../src/coax_configuration.dart';
import '../src/coax_controller.dart';

class CoaxSettingsView extends StatelessWidget {
  const CoaxSettingsView({super.key, required this.coaxManager});

  final CoaxManager coaxManager;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: coaxManager,
      builder: (context, _) {
        return AlertDialog(
          title: const Text('COAX'),
          content: SizedBox(
            width: 720,
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SegmentedButton<CoaxServerTransport>(
                    segments: [
                      for (final transport
                          in coaxManager.capabilities.coaxServerTransports)
                        ButtonSegment(
                          value: transport,
                          label: Text(coaxTransportTitle(transport)),
                        ),
                    ],
                    selected: {coaxManager.serverTransport},
                    onSelectionChanged: (values) {
                      coaxManager.setServerTransport(values.first);
                    },
                  ),
                  const SizedBox(height: 16),
                  if (coaxManager.serverTransport == CoaxServerTransport.tcp)
                    _FieldRow(
                      label: 'Host',
                      child: TextFormField(
                        initialValue: coaxManager.serverHost,
                        decoration: const InputDecoration(
                          border: OutlineInputBorder(),
                          hintText: '127.0.0.1',
                          isDense: true,
                        ),
                        onChanged: coaxManager.setServerHost,
                      ),
                    ),
                  if (coaxManager.serverTransport == CoaxServerTransport.tcp)
                    _FieldRow(
                      label: 'Port',
                      child: TextFormField(
                        initialValue: coaxManager.serverPortText,
                        decoration: InputDecoration(
                          border: const OutlineInputBorder(),
                          hintText: coaxManager.serverTransport.defaultPort
                              .toString(),
                          isDense: true,
                        ),
                        onChanged: coaxManager.setServerPortText,
                      ),
                    ),
                  if (coaxManager.serverTransport == CoaxServerTransport.unix)
                    _FieldRow(
                      label: 'Socket path',
                      child: TextFormField(
                        initialValue: coaxManager.serverUnixPath,
                        decoration: InputDecoration(
                          border: const OutlineInputBorder(),
                          hintText: coaxManager.defaultUnixPath,
                          isDense: true,
                        ),
                        onChanged: coaxManager.setServerUnixPath,
                      ),
                    ),
                  if (coaxManager.serverPortValidationMessage != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(
                        coaxManager.serverPortValidationMessage!,
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ),
                  const SizedBox(height: 16),
                  Text(
                    'Endpoint',
                    style: Theme.of(context).textTheme.labelLarge,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    coaxManager.serverStatus.endpoint ??
                        coaxManager.serverPreviewEndpoint,
                    style: Theme.of(
                      context,
                    ).textTheme.bodyMedium?.copyWith(fontFamily: 'monospace'),
                  ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Done'),
            ),
          ],
        );
      },
    );
  }
}

@Deprecated('Use CoaxSettingsView')
class CoaxSettingsDialog extends CoaxSettingsView {
  const CoaxSettingsDialog({super.key, required CoaxController controller})
    : super(coaxManager: controller);
}

class _FieldRow extends StatelessWidget {
  const _FieldRow({required this.label, required this.child});

  final String label;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        children: [
          SizedBox(
            width: 112,
            child: Text(label, style: Theme.of(context).textTheme.labelLarge),
          ),
          Expanded(child: child),
        ],
      ),
    );
  }
}
