import 'dart:async';
import 'dart:ui' show AppExitResponse;

import 'package:holons_app/holons_app.dart'
    show CoaxControlBar, CoaxController, CoaxSettingsDialog, transportTitle;
import 'package:shadcn_flutter/shadcn_flutter.dart';

import 'controller/greeting_controller.dart';
import 'gen/v1/greeting.pb.dart';
import 'model/app_model.dart';
import 'ui/speech_bubble.dart';

class GabrielGreetingApp extends StatelessWidget {
  const GabrielGreetingApp({
    super.key,
    required this.greetingController,
    required this.coaxController,
  });

  final GreetingController greetingController;
  final CoaxController coaxController;

  @override
  Widget build(BuildContext context) {
    return ShadcnApp(
      title: 'Gabriel Greeting',
      debugShowCheckedModeBanner: false,
      theme: const ThemeData(
        colorScheme: ColorSchemes.lightSlate,
        radius: 0.9,
        scaling: 1.0,
      ),
      darkTheme: const ThemeData(
        colorScheme: ColorSchemes.darkSlate,
        radius: 0.9,
        scaling: 1.0,
      ),
      themeMode: ThemeMode.system,
      home: GabrielGreetingHomePage(
        greetingController: greetingController,
        coaxController: coaxController,
      ),
    );
  }
}

class GabrielGreetingHomePage extends StatefulWidget {
  const GabrielGreetingHomePage({
    super.key,
    required this.greetingController,
    required this.coaxController,
  });

  final GreetingController greetingController;
  final CoaxController coaxController;

  @override
  State<GabrielGreetingHomePage> createState() =>
      _GabrielGreetingHomePageState();
}

class _GabrielGreetingHomePageState extends State<GabrielGreetingHomePage> {
  late final AppLifecycleListener _appLifecycleListener;
  late final TextEditingController _nameController;
  late final Listenable _listenable;
  Future<void>? _shutdownFuture;
  bool _syncListenerAttached = false;

  @override
  void initState() {
    super.initState();
    _appLifecycleListener = AppLifecycleListener(
      onExitRequested: _handleExitRequested,
    );
    _nameController = TextEditingController(
      text: widget.greetingController.userName,
    );
    _listenable = Listenable.merge(<Listenable>[
      widget.greetingController,
      widget.coaxController,
    ]);
    widget.greetingController.addListener(_syncNameField);
    _syncListenerAttached = true;
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (!mounted || _shutdownFuture != null) {
        return;
      }
      await widget.greetingController.initialize();
      if (!mounted || _shutdownFuture != null) {
        return;
      }
      await widget.coaxController.startIfEnabled();
    });
  }

  @override
  void dispose() {
    _appLifecycleListener.dispose();
    _detachSyncListener();
    unawaited(_shutdownControllers());
    _nameController.dispose();
    super.dispose();
  }

  Future<AppExitResponse> _handleExitRequested() async {
    await _shutdownControllers();
    return AppExitResponse.exit;
  }

  Future<void> _shutdownControllers() {
    final existing = _shutdownFuture;
    if (existing != null) {
      return existing;
    }
    final future = () async {
      _detachSyncListener();
      await widget.coaxController.shutdown();
      await widget.greetingController.shutdown();
    }();
    _shutdownFuture = future;
    return future;
  }

  void _detachSyncListener() {
    if (!_syncListenerAttached) {
      return;
    }
    widget.greetingController.removeListener(_syncNameField);
    _syncListenerAttached = false;
  }

  void _syncNameField() {
    final nextText = widget.greetingController.userName;
    if (_nameController.text == nextText) {
      return;
    }
    _nameController.value = TextEditingValue(
      text: nextText,
      selection: TextSelection.collapsed(offset: nextText.length),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _listenable,
      builder: (context, _) {
        final controller = widget.greetingController;
        final coax = widget.coaxController;
        final theme = Theme.of(context);
        final darkMode = theme.brightness == Brightness.dark;

        return Scaffold(
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
                colors: <Color>[
                  theme.colorScheme.background,
                  theme.colorScheme.accent.withValues(
                    alpha: darkMode ? 0.28 : 0.72,
                  ),
                  theme.colorScheme.muted.withValues(
                    alpha: darkMode ? 0.34 : 0.88,
                  ),
                ],
              ),
            ),
            child: SafeArea(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Padding(
                    padding: const EdgeInsets.fromLTRB(32, 16, 32, 16),
                    child: CoaxControlBar(
                      controller: coax,
                      onOpenSettings: () => _showCoaxSettings(context),
                    ),
                  ),
                  DecoratedBox(
                    decoration: BoxDecoration(
                      border: Border(
                        bottom: BorderSide(
                          color: theme.colorScheme.border.withValues(
                            alpha: 0.5,
                          ),
                        ),
                      ),
                    ),
                    child: const SizedBox(height: 0),
                  ),
                  Expanded(
                    child: Padding(
                      padding: const EdgeInsets.all(32),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          _WorkspaceBar(controller: controller),
                          const SizedBox(height: 32),
                          Expanded(
                            child: LayoutBuilder(
                              builder: (context, constraints) {
                                final inputWidth = (constraints.maxWidth * 0.27)
                                    .clamp(220.0, 320.0);
                                final gapWidth = (constraints.maxWidth * 0.03)
                                    .clamp(18.0, 32.0);
                                return Row(
                                  crossAxisAlignment: CrossAxisAlignment.center,
                                  children: [
                                    SizedBox(
                                      width: inputWidth,
                                      child: Column(
                                        mainAxisAlignment:
                                            MainAxisAlignment.center,
                                        crossAxisAlignment:
                                            CrossAxisAlignment.start,
                                        children: [
                                          _InputColumn(
                                            controller: controller,
                                            nameController: _nameController,
                                            width: inputWidth,
                                          ),
                                        ],
                                      ),
                                    ),
                                    SizedBox(width: gapWidth),
                                    Expanded(
                                      child: _BubbleColumn(
                                        controller: controller,
                                      ),
                                    ),
                                  ],
                                );
                              },
                            ),
                          ),
                          const SizedBox(height: 28),
                          Align(
                            alignment: Alignment.center,
                            child: SizedBox(
                              width: 260,
                              child: _LanguagePicker(controller: controller),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Future<void> _showCoaxSettings(BuildContext context) {
    return showDialog<void>(
      context: context,
      builder: (context) =>
          CoaxSettingsDialog(controller: widget.coaxController),
    );
  }
}

class _WorkspaceBar extends StatelessWidget {
  const _WorkspaceBar({required this.controller});

  final GreetingController controller;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Expanded(child: _HolonHeaderGroup(controller: controller)),
        const SizedBox(width: 32),
        _RuntimeHeaderGroup(controller: controller),
      ],
    );
  }
}

class _HolonHeaderGroup extends StatelessWidget {
  const _HolonHeaderGroup({required this.controller});

  final GreetingController controller;

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.centerLeft,
      child: SizedBox(
        width: 360,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: 260,
              child: Select<GabrielHolonIdentity>(
                value: controller.selectedHolon,
                placeholder: const Text('Loading holons...'),
                onChanged: (value) {
                  if (value != null) {
                    controller.selectHolonBySlug(value.slug);
                  }
                },
                itemBuilder: (context, value) => Text(value.displayName),
                popup: (context) => SelectPopup(
                  items: SelectItemList(
                    children: controller.availableHolons
                        .map(
                          (identity) => SelectItemButton<GabrielHolonIdentity>(
                            value: identity,
                            child: Text(identity.displayName),
                          ),
                        )
                        .toList(growable: false),
                  ),
                ),
              ),
            ),
            if (controller.selectedHolon != null)
              SelectableText(controller.selectedHolon!.slug).small().muted(),
          ],
        ).gap(8),
      ),
    );
  }
}

class _RuntimeHeaderGroup extends StatelessWidget {
  const _RuntimeHeaderGroup({required this.controller});

  final GreetingController controller;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 240,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              const Text('mode:').small().semiBold(),
              SizedBox(
                width: 140,
                child: Select<String>(
                  value: controller.transport,
                  onChanged: (value) {
                    if (value != null) {
                      controller.setTransport(value);
                    }
                  },
                  itemBuilder: (context, value) => Text(transportTitle(value)),
                  popup: (context) => SelectPopup(
                    items: SelectItemList(
                      children: controller.capabilities.appTransports
                          .map(
                            (transport) => SelectItemButton<String>(
                              value: transport,
                              child: Text(transportTitle(transport)),
                            ),
                          )
                          .toList(growable: false),
                    ),
                  ),
                ),
              ),
            ],
          ).gap(8),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              Text(controller.statusTitle).small().semiBold(),
              _RuntimeDot(
                isLoading: controller.isLoading,
                isRunning: controller.isRunning,
              ),
            ],
          ).gap(8),
        ],
      ),
    );
  }
}

class _InputColumn extends StatelessWidget {
  const _InputColumn({
    required this.controller,
    required this.nameController,
    required this.width,
  });

  final GreetingController controller;
  final TextEditingController nameController;
  final double width;

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.centerLeft,
      child: SizedBox(
        width: width,
        child: TextField(
          key: const ValueKey<String>('name-input'),
          controller: nameController,
          placeholder: const Text('World'),
          maxLines: 1,
          onChanged: (value) {
            controller.setUserName(value);
          },
        ),
      ),
    );
  }
}

class _LanguagePicker extends StatelessWidget {
  const _LanguagePicker({required this.controller});

  final GreetingController controller;

  @override
  Widget build(BuildContext context) {
    return Select<String>(
      value: controller.selectedLanguageCode.isEmpty
          ? null
          : controller.selectedLanguageCode,
      placeholder: Text(
        controller.isLoading ? 'Loading...' : 'Select language',
      ),
      onChanged: (value) {
        if (value != null) {
          controller.setSelectedLanguage(value);
        }
      },
      itemBuilder: (context, value) {
        final language = controller.availableLanguages.firstWhere(
          (item) => item.code == value,
          orElse: () => Language(code: value),
        );
        return Text(_languageTitle(language));
      },
      popup: (context) => SelectPopup(
        items: SelectItemList(
          children: controller.availableLanguages
              .map(
                (language) => SelectItemButton<String>(
                  value: language.code,
                  child: Text(_languageTitle(language)),
                ),
              )
              .toList(growable: false),
        ),
      ),
    );
  }

  String _languageTitle(Language language) {
    if (language.native.trim().isNotEmpty && language.name.trim().isNotEmpty) {
      return '${language.native} (${language.name})';
    }
    return language.code;
  }
}

class _BubbleColumn extends StatelessWidget {
  const _BubbleColumn({required this.controller});

  final GreetingController controller;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final borderColor = theme.colorScheme.border.withValues(alpha: 0.72);
    final bubbleColor = theme.colorScheme.card;

    return SizedBox.expand(
      child: Stack(
        children: [
          Positioned.fill(
            child: ClipPath(
              clipper: const LeftPointerBubbleClipper(),
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: bubbleColor,
                  border: Border.all(color: borderColor, width: 1.5),
                  boxShadow: <BoxShadow>[
                    BoxShadow(
                      color: theme.colorScheme.foreground.withValues(
                        alpha: theme.brightness == Brightness.dark
                            ? 0.10
                            : 0.05,
                      ),
                      blurRadius: 18,
                      offset: const Offset(0, 8),
                    ),
                  ],
                ),
                child: const SizedBox.expand(),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(40, 32, 32, 32),
            child: Center(child: _BubbleContent(controller: controller)),
          ),
        ],
      ),
    );
  }
}

class _BubbleContent extends StatelessWidget {
  const _BubbleContent({required this.controller});

  final GreetingController controller;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (controller.connectionError != null) {
      return _ErrorPanel(
        title: 'Holon Offline',
        message: controller.connectionError!,
      );
    }
    if (controller.error != null) {
      return _ErrorPanel(title: 'Error', message: controller.error!);
    }
    final greeting = controller.greeting.isEmpty && controller.isGreeting
        ? '...'
        : controller.greeting;
    return SelectableText(
      greeting,
      textAlign: TextAlign.center,
      style: theme.typography.x4Large.copyWith(
        fontWeight: FontWeight.w500,
        color: theme.colorScheme.foreground,
      ),
    );
  }
}

class _ErrorPanel extends StatelessWidget {
  const _ErrorPanel({required this.title, required this.message});

  final String title;
  final String message;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(LucideIcons.triangleAlert, color: Color(0xFFD84A4A)),
            Text(title).large().semiBold(),
          ],
        ).gap(10),
        SelectableText(message).small(),
      ],
    ).gap(12);
  }
}

class _RuntimeDot extends StatelessWidget {
  const _RuntimeDot({required this.isLoading, required this.isRunning});

  final bool isLoading;
  final bool isRunning;

  @override
  Widget build(BuildContext context) {
    final color = isLoading
        ? const Color(0xFFD2A243)
        : isRunning
        ? const Color(0xFF66B85E)
        : const Color(0xFFD96A6A);
    return Container(
      width: 10,
      height: 10,
      decoration: BoxDecoration(color: color, shape: BoxShape.circle),
    );
  }
}
