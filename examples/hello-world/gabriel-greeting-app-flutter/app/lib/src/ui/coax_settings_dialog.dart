import 'package:shadcn_flutter/shadcn_flutter.dart';

import '../controller/coax_controller.dart';
import '../model/app_model.dart';

class CoaxSettingsDialog extends StatelessWidget {
  const CoaxSettingsDialog({super.key, required this.controller});

  final CoaxController controller;

  @override
  Widget build(BuildContext context) {
    final transportItems = controller.capabilities.coaxServerTransports
        .map(
          (transport) => SelectItemButton<CoaxServerTransport>(
            value: transport,
            child: Text(coaxTransportTitle(transport)),
          ),
        )
        .toList(growable: false);

    return AnimatedBuilder(
      animation: controller,
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
                            value: controller.serverTransport,
                            onChanged: (value) {
                              if (value != null) {
                                controller.setServerTransport(value);
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
                        if (controller.serverTransport ==
                            CoaxServerTransport.tcp)
                          _LabeledRow(
                            label: 'Host',
                            child: TextField(
                              initialValue: controller.serverHost,
                              placeholder: const Text('127.0.0.1'),
                              onChanged: controller.setServerHost,
                            ),
                          ),
                        if (controller.serverTransport ==
                            CoaxServerTransport.tcp)
                          _LabeledRow(
                            label: 'Port',
                            child: TextField(
                              initialValue: controller.serverPortText,
                              placeholder: Text(
                                controller.serverTransport.defaultPort
                                    .toString(),
                              ),
                              onChanged: controller.setServerPortText,
                            ),
                          ),
                        if (controller.serverTransport ==
                            CoaxServerTransport.unix)
                          _LabeledRow(
                            label: 'Socket path',
                            child: TextField(
                              initialValue: controller.serverUnixPath,
                              placeholder: Text(
                                CoaxSettingsSnapshot.defaultUnixPath,
                              ),
                              onChanged: controller.setServerUnixPath,
                            ),
                          ),
                        if (controller.serverPortValidationMessage != null)
                          Text(
                            controller.serverPortValidationMessage!,
                          ).small().muted(),
                        const SizedBox(height: 12),
                        Card(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('Endpoint').semiBold(),
                              SelectableText(
                                controller.serverStatus.endpoint ??
                                    controller.serverPreviewEndpoint,
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
