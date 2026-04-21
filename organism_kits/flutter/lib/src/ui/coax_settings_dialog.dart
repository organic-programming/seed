import 'package:shadcn_flutter/shadcn_flutter.dart';

import '../coax_configuration.dart';
import '../coax_controller.dart';

class CoaxSettingsView extends StatelessWidget {
  const CoaxSettingsView({super.key, required this.coaxManager});

  final CoaxManager coaxManager;

  @override
  Widget build(BuildContext context) {
    final transportItems = coaxManager.capabilities.coaxServerTransports
        .map(
          (transport) => SelectItemButton<CoaxServerTransport>(
            value: transport,
            child: Text(coaxTransportTitle(transport)),
          ),
        )
        .toList(growable: false);

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
                  _Section(
                    title: 'Server',
                    subtitle: 'Expose the embedded runtime directly.',
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _LabeledRow(
                          label: 'Transport',
                          child: Select<CoaxServerTransport>(
                            value: coaxManager.serverTransport,
                            onChanged: (value) {
                              if (value != null) {
                                coaxManager.setServerTransport(value);
                              }
                            },
                            itemBuilder: (context, value) {
                              return Text(coaxTransportTitle(value));
                            },
                            popup: (context) => SelectPopup(
                              items: SelectItemList(children: transportItems),
                            ),
                          ),
                        ),
                        if (coaxManager.serverTransport ==
                            CoaxServerTransport.tcp)
                          _LabeledRow(
                            label: 'Host',
                            child: TextField(
                              initialValue: coaxManager.serverHost,
                              placeholder: const Text('127.0.0.1'),
                              onChanged: coaxManager.setServerHost,
                            ),
                          ),
                        if (coaxManager.serverTransport ==
                            CoaxServerTransport.tcp)
                          _LabeledRow(
                            label: 'Port',
                            child: TextField(
                              initialValue: coaxManager.serverPortText,
                              placeholder: Text(
                                coaxManager.serverTransport.defaultPort
                                    .toString(),
                              ),
                              onChanged: coaxManager.setServerPortText,
                            ),
                          ),
                        if (coaxManager.serverTransport ==
                            CoaxServerTransport.unix)
                          _LabeledRow(
                            label: 'Socket path',
                            child: TextField(
                              initialValue: coaxManager.serverUnixPath,
                              placeholder: Text(
                                coaxManager.defaultUnixPath,
                              ),
                              onChanged: coaxManager.setServerUnixPath,
                            ),
                          ),
                        if (coaxManager.serverPortValidationMessage != null)
                          Text(
                            coaxManager.serverPortValidationMessage!,
                          ).small().muted(),
                        const SizedBox(height: 12),
                        Card(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('Endpoint').semiBold(),
                              SelectableText(
                                coaxManager.serverStatus.endpoint ??
                                    coaxManager.serverPreviewEndpoint,
                              ).small(),
                            ],
                          ).gap(8),
                        ),
                      ],
                    ).gap(12),
                  ),
                ],
              ).gap(16),
            ),
          ),
          actions: [
            OutlineButton(
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
  const CoaxSettingsDialog({
    super.key,
    required CoaxController controller,
  }) : super(coaxManager: controller);
}

class _Section extends StatelessWidget {
  const _Section({
    required this.title,
    required this.subtitle,
    required this.child,
  });

  final String title;
  final String subtitle;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title).large().semiBold(),
          Text(subtitle).small().muted(),
          const Divider(),
          child,
        ],
      ).gap(10),
    );
  }
}

class _LabeledRow extends StatelessWidget {
  const _LabeledRow({required this.label, required this.child});

  final String label;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        SizedBox(width: 110, child: Text(label).small().semiBold()),
        Expanded(child: child),
      ],
    ).gap(12);
  }
}
