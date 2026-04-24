import 'package:flutter/material.dart';

import '../src/coax_configuration.dart';
import '../src/coax_controller.dart';

class CoaxControlsView extends StatelessWidget {
  const CoaxControlsView({
    super.key,
    required this.coaxManager,
    required this.onOpenSettings,
  });

  final CoaxManager coaxManager;
  final VoidCallback onOpenSettings;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: coaxManager,
      builder: (context, _) {
        final endpoint = coaxManager.serverStatus.endpoint;
        return Wrap(
          crossAxisAlignment: WrapCrossAlignment.center,
          alignment: WrapAlignment.end,
          spacing: 8,
          runSpacing: 6,
          children: [
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Switch(
                  value: coaxManager.isEnabled,
                  onChanged: coaxManager.setEnabled,
                ),
                const Text('COAX'),
                IconButton(
                  tooltip: 'COAX settings',
                  onPressed: onOpenSettings,
                  icon: const Icon(Icons.tune),
                ),
              ],
            ),
            if (endpoint != null)
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SelectableText(
                    endpoint,
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  const SizedBox(width: 8),
                  CoaxSurfaceBadge(state: coaxManager.serverStatus.state),
                ],
              ),
            if (coaxManager.statusDetail != null)
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 360),
                child: Text(
                  coaxManager.statusDetail!,
                  textAlign: TextAlign.end,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
          ],
        );
      },
    );
  }
}

@Deprecated('Use CoaxControlsView')
class CoaxControlBar extends CoaxControlsView {
  const CoaxControlBar({
    super.key,
    required CoaxController controller,
    required VoidCallback onOpenSettings,
  }) : super(coaxManager: controller, onOpenSettings: onOpenSettings);
}

class CoaxSurfaceBadge extends StatelessWidget {
  const CoaxSurfaceBadge({super.key, required this.state});

  final CoaxSurfaceState state;

  @override
  Widget build(BuildContext context) {
    final color = switch (state) {
      CoaxSurfaceState.live => Colors.green,
      CoaxSurfaceState.error => Colors.red,
      CoaxSurfaceState.announced => Colors.orange,
      CoaxSurfaceState.saved => Colors.blueGrey,
      CoaxSurfaceState.off => Colors.grey,
    };
    return Text(
      state.badgeTitle,
      style: Theme.of(context).textTheme.labelSmall?.copyWith(
        color: color,
        fontWeight: FontWeight.w700,
      ),
    );
  }
}
