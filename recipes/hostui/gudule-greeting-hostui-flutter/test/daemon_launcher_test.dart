import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

import 'package:gudule_greeting_hostui_flutter/src/client/daemon_launcher.dart';
import 'package:gudule_greeting_hostui_flutter/src/client/greeting_target.dart';

void main() {
  group('GreetingTargetResolver', () {
    test('compile-time GREETING_TARGET wins over the process environment', () {
      final resolver = GreetingTargetResolver(
        compileTimeTarget: 'tcp://127.0.0.1:9091',
        environment: const {'GREETING_TARGET': 'tcp://127.0.0.1:9092'},
        executablePath: '/tmp/app',
        currentDirectoryPath: '/tmp',
      );

      final endpoint = resolver.resolve();
      expect(endpoint.target, 'tcp://127.0.0.1:9091');
      expect(endpoint.bundledBinaryPath, isNull);
    });

    test('falls back to a local built daemon when no target is configured',
        () async {
      final sandbox = await Directory.systemTemp.createTemp('greeting-target-');
      addTearDown(() => sandbox.delete(recursive: true));

      final daemon = File(
        '${sandbox.path}/build/${GreetingTargetResolver.daemonBinary}',
      );
      await daemon.parent.create(recursive: true);
      await daemon.writeAsString('daemon');

      final resolver = GreetingTargetResolver(
        environment: const {},
        executablePath: '${sandbox.path}/app',
        currentDirectoryPath: sandbox.path,
      );

      final endpoint = resolver.resolve();
      expect(endpoint.target, isNull);
      expect(endpoint.bundledBinaryPath, daemon.path);
    });
  });

  group('DaemonLauncher', () {
    test('stages a temporary holon and connects to the bundled daemon slug',
        () async {
      final sandbox = await Directory.systemTemp.createTemp('daemon-launcher-');
      addTearDown(() => sandbox.delete(recursive: true));

      final daemon = File(
        '${sandbox.path}/${GreetingTargetResolver.daemonBinary}',
      );
      await daemon.writeAsString('daemon');

      final stagedRoot = Directory('${sandbox.path}/stage');
      String currentDirectory = sandbox.path;
      String? connectTarget;
      holons.ConnectOptions? connectOptions;
      int? killedPid;
      final channel = ClientChannel('localhost', port: 1);

      final launcher = DaemonLauncher(
        connect: (target, [opts]) async {
          connectTarget = target;
          connectOptions = opts;
          return channel;
        },
        disconnect: (_) async {},
        createTempDirectory: (_) async {
          await stagedRoot.create(recursive: true);
          return stagedRoot;
        },
        getCurrentDirectory: () => currentDirectory,
        setCurrentDirectory: (path) => currentDirectory = path,
        killPid: (pid) async => killedPid = pid,
      );

      final launched = await launcher.start(bundledBinaryPath: daemon.path);
      expect(launched, same(channel));
      expect(connectTarget, GreetingTargetResolver.daemonSlug);
      expect(connectOptions?.transport, 'tcp');
      expect(currentDirectory, sandbox.path);

      final holonDir = Directory(
        '${stagedRoot.path}/holons/${GreetingTargetResolver.daemonSlug}',
      );
      expect(holonDir.existsSync(), isTrue);
      expect(
        File('${holonDir.path}/holon.yaml').readAsStringSync(),
        contains('family_name: "Greeting-Daemon-Go"'),
      );

      if (!Platform.isWindows) {
        await File('${holonDir.path}/daemon.pid').writeAsString('4242');
      }

      await launcher.stop(channel);
      expect(stagedRoot.existsSync(), isFalse);
      if (!Platform.isWindows) {
        expect(killedPid, 4242);
      }
    });

    test('cleans up the staged holon when connect fails', () async {
      final sandbox = await Directory.systemTemp.createTemp('daemon-launcher-');
      addTearDown(() => sandbox.delete(recursive: true));

      final daemon = File(
        '${sandbox.path}/${GreetingTargetResolver.daemonBinary}',
      );
      await daemon.writeAsString('daemon');

      final stagedRoot = Directory('${sandbox.path}/stage');

      final launcher = DaemonLauncher(
        connect: (_, [__]) async => throw StateError('connect failed'),
        disconnect: (_) async {},
        createTempDirectory: (_) async {
          await stagedRoot.create(recursive: true);
          return stagedRoot;
        },
        getCurrentDirectory: () => sandbox.path,
        setCurrentDirectory: (_) {},
        killPid: (_) async {},
      );

      await expectLater(
        launcher.start(bundledBinaryPath: daemon.path),
        throwsA(isA<StateError>()),
      );
      expect(stagedRoot.existsSync(), isFalse);
      expect(launcher.stagedRootPath, isNull);
    });
  });
}
