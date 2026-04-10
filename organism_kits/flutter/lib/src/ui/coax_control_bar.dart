import 'package:shadcn_flutter/shadcn_flutter.dart';

import '../coax_configuration.dart';
import '../coax_controller.dart';

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
        return Row(
          children: [
            const Spacer(),
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Row(
                  children: [
                    Switch(
                      value: coaxManager.isEnabled,
                      onChanged: coaxManager.setEnabled,
                      trailing: const Text('COAX').small().semiBold(),
                    ),
                    GhostButton(
                      onPressed: onOpenSettings,
                      density: ButtonDensity.icon,
                      child: const Icon(LucideIcons.settings2),
                    ),
                  ],
                ).gap(8),
                if (coaxManager.serverStatus.endpoint != null)
                  Row(
                    children: [
                      const Text('Server:').small().muted(),
                      SelectableText(
                        coaxManager.serverStatus.endpoint!,
                      ).small().muted(),
                      CoaxSurfaceBadge(state: coaxManager.serverStatus.state),
                    ],
                  ).gap(8),
                if (coaxManager.statusDetail != null)
                  SizedBox(
                    width: 360,
                    child: Text(
                      coaxManager.statusDetail!,
                      textAlign: TextAlign.right,
                    ).small().muted(),
                  ),
              ],
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
  }) : super(
         coaxManager: controller,
         onOpenSettings: onOpenSettings,
       );
}

class CoaxSurfaceBadge extends StatelessWidget {
  const CoaxSurfaceBadge({super.key, required this.state});

  final CoaxSurfaceState state;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = switch (state) {
      CoaxSurfaceState.live => const Color(0xFF66B85E),
      CoaxSurfaceState.error => const Color(0xFFD96A6A),
      CoaxSurfaceState.announced => theme.colorScheme.mutedForeground,
      CoaxSurfaceState.saved => theme.colorScheme.mutedForeground,
      CoaxSurfaceState.off => theme.colorScheme.mutedForeground,
    };
    return Text(
      state.badgeTitle,
      style: theme.typography.small.copyWith(
        fontWeight: FontWeight.w600,
        color: color,
      ),
    );
  }
}
