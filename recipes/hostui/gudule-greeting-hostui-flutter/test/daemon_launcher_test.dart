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
      expect(endpoint.daemon, isNull);
    });

    test('falls back to a local built daemon when no target is configured',
        () async {
      final sandbox = await Directory.systemTemp.createTemp('greeting-target-');
      addTearDown(() => sandbox.delete(recursive: true));

      final daemon = File('${sandbox.path}/build/gudule-daemon-greeting-rust');
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
      expect(endpoint.daemon?.slug, 'gudule-greeting-daemon-rust');
      expect(endpoint.daemon?.familyName, 'Greeting-Daemon-Rust');
    });

    test('resolves assembly family and transport defaults', () {
      expect(
        resolveGreetingAssemblyFamily(
            const {'OP_ASSEMBLY_FAMILY': 'Greeting-Flutter-Rust'}),
        'Greeting-Flutter-Rust',
      );
      expect(resolveGreetingAssemblyFamily(const {}), 'Greeting-Flutter-Go');
      expect(
        resolveGreetingTransport(const {'OP_ASSEMBLY_TRANSPORT': 'tcp'}),
        'tcp',
      );
      expect(resolveGreetingTransport(const {}), 'stdio');
    });

    test('derives assembly family from the bundled daemon identity', () {
      const endpoint = GreetingEndpoint(
        daemon: GreetingDaemonIdentity(
          slug: 'gudule-greeting-daemon-rust',
          binaryName: 'gudule-daemon-greeting-rust',
          familyName: 'Greeting-Daemon-Rust',
        ),
      );

      expect(
        deriveGreetingAssemblyFamilyFromEndpoint(endpoint),
        'Greeting-Flutter-Rust',
      );
    });
  });

  group('DaemonLauncher', () {
    test('stages a temporary holon and connects to the bundled daemon slug',
        () async {
      final sandbox = await Directory.systemTemp.createTemp('daemon-launcher-');
      addTearDown(() => sandbox.delete(recursive: true));

      final daemon = File('${sandbox.path}/gudule-daemon-greeting-rust');
      await daemon.writeAsString('daemon');

      final stagedRoot = Directory('${sandbox.path}/stage');
      String currentDirectory = sandbox.path;
      String? connectTarget;
      holons.ConnectOptions? connectOptions;
      String? startedBinaryPath;
      String? startedPortFilePath;
      bool sessionStopped = false;
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
        getEnvironment: () => const {'OP_ASSEMBLY_TRANSPORT': 'tcp'},
        startBundledDaemon: (binaryPath, portFilePath) async {
          startedBinaryPath = binaryPath;
          startedPortFilePath = portFilePath;
          await File(portFilePath).parent.create(recursive: true);
          await File(portFilePath).writeAsString('tcp://127.0.0.1:43123\n');
          return BundledDaemonSession(
            stop: () async => sessionStopped = true,
          );
        },
      );

      final launched = await launcher.start(
        GreetingEndpoint(
          bundledBinaryPath: daemon.path,
          daemon: GreetingDaemonIdentity.fromBinaryPath(daemon.path),
        ),
      );
      expect(launched, same(channel));
      expect(connectTarget, 'gudule-greeting-daemon-rust');
      expect(connectOptions?.transport, 'tcp');
      expect(connectOptions?.start, isFalse);
      expect(connectOptions?.portFile, startedPortFilePath);
      expect(currentDirectory, sandbox.path);
      expect(startedBinaryPath, daemon.absolute.path);

      final holonDir = Directory(
        '${stagedRoot.path}/holons/gudule-greeting-daemon-rust',
      );
      expect(holonDir.existsSync(), isTrue);
      expect(
        File('${holonDir.path}/holon.yaml').readAsStringSync(),
        contains('family_name: "Greeting-Daemon-Rust"'),
      );
      expect(
        File('${holonDir.path}/holon.yaml').readAsStringSync(),
        contains('binary: "${daemon.absolute.path}"'),
      );
      expect(File(startedPortFilePath!).existsSync(), isTrue);

      await launcher.stop(channel);
      expect(stagedRoot.existsSync(), isFalse);
      expect(sessionStopped, isTrue);
    });

    test('cleans up the staged holon when connect fails', () async {
      final sandbox = await Directory.systemTemp.createTemp('daemon-launcher-');
      addTearDown(() => sandbox.delete(recursive: true));

      final daemon = File('${sandbox.path}/gudule-daemon-greeting-rust');
      await daemon.writeAsString('daemon');

      final stagedRoot = Directory('${sandbox.path}/stage');

      bool sessionStopped = false;

      final launcher = DaemonLauncher(
        connect: (_, [__]) async => throw StateError('connect failed'),
        disconnect: (_) async {},
        createTempDirectory: (_) async {
          await stagedRoot.create(recursive: true);
          return stagedRoot;
        },
        getCurrentDirectory: () => sandbox.path,
        setCurrentDirectory: (_) {},
        getEnvironment: () => const {'OP_ASSEMBLY_TRANSPORT': 'tcp'},
        startBundledDaemon: (_, portFilePath) async {
          await File(portFilePath).parent.create(recursive: true);
          await File(portFilePath).writeAsString('tcp://127.0.0.1:43123\n');
          return BundledDaemonSession(
            stop: () async => sessionStopped = true,
          );
        },
      );

      await expectLater(
        launcher.start(
          GreetingEndpoint(
            bundledBinaryPath: daemon.path,
            daemon: GreetingDaemonIdentity.fromBinaryPath(daemon.path),
          ),
        ),
        throwsA(isA<StateError>()),
      );
      expect(stagedRoot.existsSync(), isFalse);
      expect(launcher.stagedRootPath, isNull);
      expect(sessionStopped, isTrue);
    });

    test('uses Dart holons stdio connect path when transport override is stdio',
        () async {
      final sandbox = await Directory.systemTemp.createTemp('daemon-launcher-');
      addTearDown(() => sandbox.delete(recursive: true));

      final daemon = File('${sandbox.path}/gudule-daemon-greeting-rust');
      await daemon.writeAsString('daemon');

      final stagedRoot = Directory('${sandbox.path}/stage');
      String currentDirectory = sandbox.path;
      String? connectTarget;
      holons.ConnectOptions? connectOptions;
      bool startBundledDaemonCalled = false;
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
        getEnvironment: () => const {'OP_ASSEMBLY_TRANSPORT': 'stdio'},
        startBundledDaemon: (_, __) async {
          startBundledDaemonCalled = true;
          return const BundledDaemonSession(stop: _noopStop);
        },
      );

      final launched = await launcher.start(
        GreetingEndpoint(
          bundledBinaryPath: daemon.path,
          daemon: GreetingDaemonIdentity.fromBinaryPath(daemon.path),
        ),
      );

      expect(launched, same(channel));
      expect(connectTarget, 'gudule-greeting-daemon-rust');
      expect(connectOptions?.transport, 'stdio');
      expect(connectOptions?.start, isTrue);
      expect(connectOptions?.portFile, isEmpty);
      expect(startBundledDaemonCalled, isFalse);
      expect(currentDirectory, sandbox.path);
    });
  });
}

Future<void> _noopStop() async {}
