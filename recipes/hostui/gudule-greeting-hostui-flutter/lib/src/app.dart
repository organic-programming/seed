import 'dart:async';
import 'dart:ui' show AppExitResponse;

import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

import 'client/greeting_client.dart';
import 'client/greeting_target.dart';
import 'screens/greeting_screen.dart';

class GreetingApp extends StatefulWidget {
  const GreetingApp({super.key});

  @override
  State<GreetingApp> createState() => _GreetingAppState();
}

class _GreetingAppState extends State<GreetingApp> {
  final GreetingClient _client = GreetingClient();
  final GreetingTargetResolver _targetResolver = GreetingTargetResolver();
  late final AppLifecycleListener _lifecycleListener;
  bool _connecting = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _lifecycleListener = AppLifecycleListener(
      onExitRequested: _handleExitRequested,
      onDetach: _handleDetached,
    );
    _connect();
  }

  Future<void> _connect() async {
    try {
      final endpoint = _targetResolver.resolve();
      if (endpoint.target == null && endpoint.bundledBinaryPath == null) {
        throw StateError(
          'No GreetingService target found. Set GREETING_TARGET or bundle the Go daemon.',
        );
      }
      await _client.connect(endpoint);
      setState(() => _connecting = false);
    } catch (e) {
      setState(() {
        _connecting = false;
        _error = e.toString();
      });
    }
  }

  Future<AppExitResponse> _handleExitRequested() async {
    await _client.close();
    return AppExitResponse.exit;
  }

  void _handleDetached() {
    unawaited(_client.close());
  }

  @override
  void dispose() {
    _lifecycleListener.dispose();
    unawaited(_client.close());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Flutter Greeting',
      debugShowCheckedModeBanner: false,
      theme: ThemeData.dark().copyWith(
        textTheme: GoogleFonts.interTextTheme(ThemeData.dark().textTheme),
      ),
      home: _buildHome(),
    );
  }

  Widget _buildHome() {
    if (_connecting) {
      return const Scaffold(
        backgroundColor: Color(0xFF0F0F1A),
        body: Center(child: CircularProgressIndicator()),
      );
    }
    if (_error != null) {
      return Scaffold(
        backgroundColor: const Color(0xFF0F0F1A),
        body: Center(
          child: Text(
            'Connection error:\n$_error',
            textAlign: TextAlign.center,
            style: const TextStyle(color: Colors.redAccent),
          ),
        ),
      );
    }
    return GreetingScreen(client: _client);
  }
}
