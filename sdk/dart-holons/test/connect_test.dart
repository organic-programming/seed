import 'dart:io';

import 'package:holons/holons.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:holons/src/connect.dart' as connect_impl;
import 'package:holons/src/discover.dart' as discover_impl;
import 'package:holons/src/discovery_probe.dart' show currentArchDirectory;
import 'package:test/test.dart';

import 'test_support/discovery_fixture.dart';

void main() {
  final sdkRoot = Directory.current.path;

  tearDown(() {
    discover_impl.resetDiscoveryTestOverrides();
    connect_impl.resetConnectTestOverrides();
  });

  group('connect', () {
    test('unresolvable target', () async {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      final result =
          await connect(LOCAL, 'missing', fixture.root.path, CWD, 1000)
              as ConnectResult;

      expect(result.channel, isNull);
      expect(result.origin, isNull);
      expect(result.error, equals('holon "missing" not found'));
    });

    test(
      'returns ConnectResult',
      () async {
        final helper = buildDescribeHelper(sdkRoot);
        if (helper == null) {
          return;
        }
        final fixture = createRuntimeFixture(sdkRoot);
        addTearDown(() => deleteFixture(fixture));
        configureDiscoveryRuntime(fixture);

        writeDescribePackageHolon(
          Directory('${fixture.root.path}/connect-alpha.holon'),
          executablePath: helper,
          slug: 'connect-alpha',
          entrypoint: 'connect-alpha',
        );

        final result =
            await connect(LOCAL, 'connect-alpha', fixture.root.path, CWD, 5000)
                as ConnectResult;
        addTearDown(() => disconnect(result));

        expect(result.error, isNull);
        expect(result.channel, isNotNull);

        final response =
            await HolonMetaClient(result.channel).describe(DescribeRequest());
        expect(response.manifest.identity.givenName, equals('connect-alpha'));
      },
      skip: Platform.isWindows
          ? 'fixture binary uses a POSIX shell launcher'
          : false,
    );

    test(
      'populates origin',
      () async {
        final helper = buildDescribeHelper(sdkRoot);
        if (helper == null) {
          return;
        }
        final fixture = createRuntimeFixture(sdkRoot);
        addTearDown(() => deleteFixture(fixture));
        configureDiscoveryRuntime(fixture);

        final packageDir = Directory('${fixture.root.path}/origin-alpha.holon');
        writeDescribePackageHolon(
          packageDir,
          executablePath: helper,
          slug: 'origin-alpha',
          entrypoint: 'origin-alpha',
        );

        final result =
            await connect(LOCAL, 'origin-alpha', fixture.root.path, CWD, 5000)
                as ConnectResult;
        addTearDown(() => disconnect(result));

        expect(result.error, isNull);
        expect(result.origin, isNotNull);
        expect(
            result.origin?.url, equals(Uri.file(packageDir.path).toString()));
        expect(result.origin?.info?.slug, equals('origin-alpha'));
      },
      skip: Platform.isWindows
          ? 'fixture binary uses a POSIX shell launcher'
          : false,
    );

    test(
      'disconnect accepts ConnectResult',
      () async {
        final helper = buildDescribeHelper(sdkRoot);
        if (helper == null) {
          return;
        }
        final fixture = createRuntimeFixture(sdkRoot);
        addTearDown(() => deleteFixture(fixture));
        configureDiscoveryRuntime(fixture);

        writeDescribePackageHolon(
          Directory('${fixture.root.path}/disconnect-alpha.holon'),
          executablePath: helper,
          slug: 'disconnect-alpha',
          entrypoint: 'disconnect-alpha',
        );

        final result = await connect(
          LOCAL,
          'disconnect-alpha',
          fixture.root.path,
          CWD,
          5000,
        ) as ConnectResult;

        expect(result.error, isNull);
        disconnect(result);
        await Future<void>.delayed(const Duration(milliseconds: 100));
      },
      skip: Platform.isWindows
          ? 'fixture binary uses a POSIX shell launcher'
          : false,
    );

    test(
      'package connect honors holon entrypoint before metadata files',
      () async {
        final helper = buildDescribeHelper(sdkRoot);
        if (helper == null) {
          return;
        }
        final fixture = createRuntimeFixture(sdkRoot);
        addTearDown(() => deleteFixture(fixture));
        configureDiscoveryRuntime(fixture);

        final packageDir =
            Directory('${fixture.root.path}/connect-entrypoint.holon');
        writePackageHolon(
          packageDir,
          slug: 'connect-entrypoint',
          uuid: 'uuid-connect-entrypoint',
          entrypoint: 'connect-entrypoint',
        );
        final archDir =
            Directory('${packageDir.path}/bin/${currentArchDirectory()}')
              ..createSync(recursive: true);
        File('${archDir.path}/describe_generated.json')
            .writeAsStringSync('{"note":"not executable"}\n');
        final launcher = File('${archDir.path}/connect-entrypoint');
        launcher.writeAsStringSync('''
#!/bin/sh
exec ${_shellQuote(helper)} --slug connect-entrypoint "\$@"
''');
        if (!Platform.isWindows) {
          Process.runSync('chmod', <String>['755', launcher.path]);
        }

        final result = await connect(
          LOCAL,
          'connect-entrypoint',
          fixture.root.path,
          CWD,
          5000,
        ) as ConnectResult;
        addTearDown(() => disconnect(result));

        expect(result.error, isNull);
        final response =
            await HolonMetaClient(result.channel).describe(DescribeRequest());
        expect(
            response.manifest.identity.givenName, equals('connect-entrypoint'));
      },
      skip: Platform.isWindows
          ? 'fixture binary uses a POSIX shell launcher'
          : false,
    );

    test('default port file path prefers OPPATH over cwd root', () {
      connect_impl.connectEnvironmentProvider = () => <String, String>{
            'OPPATH': '/tmp/op-home',
            'HOME': '/Users/example',
          };
      connect_impl.connectCurrentRootProvider = () => '/';

      expect(
        connect_impl.defaultPortFilePathForTest(
          'gabriel-greeting-go',
          transport: 'tcp',
        ),
        equals('/tmp/op-home/run/gabriel-greeting-go.tcp.port'),
      );
      expect(
        connect_impl.defaultPortFilePathForTest(
          'gabriel-greeting-go',
          transport: 'unix',
        ),
        equals('/tmp/op-home/run/gabriel-greeting-go.unix.port'),
      );
    });

    test('default port file path falls back to HOME/.op', () {
      connect_impl.connectEnvironmentProvider = () => <String, String>{
            'HOME': '/Users/example',
          };
      connect_impl.connectCurrentRootProvider = () => '/';

      expect(
        connect_impl.defaultPortFilePathForTest(
          'gabriel-greeting-go',
          transport: 'tcp',
        ),
        equals('/Users/example/.op/run/gabriel-greeting-go.tcp.port'),
      );
    });

    test('default port file path falls back to system temp when HOME is absent',
        () {
      connect_impl.connectEnvironmentProvider = () => <String, String>{};
      connect_impl.connectCurrentRootProvider = () => '/';

      expect(
        connect_impl.defaultPortFilePathForTest(
          'gabriel-greeting-go',
          transport: 'tcp',
        ),
        equals(
          '${Directory.systemTemp.path}/.op/run/gabriel-greeting-go.tcp.port',
        ),
      );
    });

    test('default port file path uses system temp when running from a bundle',
        () {
      final publishedRoot =
          Directory.systemTemp.createTempSync('holons-bundle-');
      addTearDown(() => publishedRoot.deleteSync(recursive: true));
      discover_impl.discoverPublishedHolonsRootProvider =
          () => publishedRoot.path;
      connect_impl.connectEnvironmentProvider = () => <String, String>{
            'OPPATH': '/tmp/op-home',
            'HOME': '/Users/example',
          };

      expect(
        connect_impl.defaultPortFilePathForTest(
          'gabriel-greeting-go',
          transport: 'unix',
        ),
        equals(
          '${Directory.systemTemp.path}${Platform.pathSeparator}holons${Platform.pathSeparator}run${Platform.pathSeparator}gabriel-greeting-go.unix.port',
        ),
      );
    });

    test('default unix socket URI uses system temp', () {
      final uri = connect_impl.defaultUnixSocketURIForTest(
        'gabriel-greeting-go',
        '/tmp/gabriel-greeting-go.port',
      );
      final path = uri.substring('unix://'.length);
      expect(
        uri,
        startsWith('unix://'),
      );
      expect(path, endsWith('.s'));
      expect(_isWithinTempHierarchy(path), isTrue);
    });

    test('default working directory falls back to system temp when read only',
        () {
      final sandbox = Directory.systemTemp.createTempSync('holons-cwd-');
      final readOnly =
          Directory('${sandbox.path}${Platform.pathSeparator}readonly')
            ..createSync(recursive: true);
      Process.runSync('chmod', <String>['555', readOnly.path]);
      addTearDown(() {
        Process.runSync('chmod', <String>['755', readOnly.path]);
        sandbox.deleteSync(recursive: true);
      });

      final resolved = connect_impl.defaultWorkingDirectoryForTest(
        readOnly.path,
        '${readOnly.path}${Platform.pathSeparator}bin${Platform.pathSeparator}demo',
      );
      expect(resolved, equals(Directory.systemTemp.absolute.path));
    },
        skip: Platform.isWindows
            ? 'chmod-based writability check is POSIX only'
            : false);

    test(
      'persistent tcp and unix transports use separate cache files',
      () async {
        if (Platform.isWindows) {
          return;
        }

        final helper = buildDescribeHelper(sdkRoot);
        if (helper == null) {
          return;
        }

        final fixture = createRuntimeFixture(sdkRoot);
        addTearDown(() => deleteFixture(fixture));
        configureDiscoveryRuntime(fixture);
        connect_impl.connectEnvironmentProvider = () => <String, String>{
              'HOME': fixture.sandbox.path,
              'OPPATH': fixture.opHome.path,
            };
        connect_impl.connectCurrentRootProvider = () => fixture.root.path;

        writePackageHolon(
          Directory('${fixture.root.path}/transport-alpha.holon'),
          slug: 'transport-alpha',
          uuid: 'uuid-transport-alpha',
          entrypoint: 'transport-alpha',
        );
        writeDescribePackageHolon(
          Directory('${fixture.root.path}/transport-alpha.holon'),
          executablePath: helper,
          slug: 'transport-alpha',
          entrypoint: 'transport-alpha',
        );

        final tcpChannel = await connect(
          'transport-alpha',
          const ConnectOptions(transport: 'tcp'),
        );
        addTearDown(() => disconnect(tcpChannel));
        final tcpDescribe =
            await HolonMetaClient(tcpChannel).describe(DescribeRequest());
        expect(
            tcpDescribe.manifest.identity.givenName, equals('transport-alpha'));

        final tcpPortFile = File(
          connect_impl.defaultPortFilePathForTest(
            'transport-alpha',
            transport: 'tcp',
          ),
        );
        expect(tcpPortFile.existsSync(), isTrue);
        expect(
          tcpPortFile.readAsStringSync().trim(),
          startsWith('tcp://127.0.0.1:'),
        );

        final unixChannel = await connect(
          'transport-alpha',
          const ConnectOptions(transport: 'unix'),
        );
        addTearDown(() => disconnect(unixChannel));
        final unixDescribe =
            await HolonMetaClient(unixChannel).describe(DescribeRequest());
        expect(unixDescribe.manifest.identity.givenName,
            equals('transport-alpha'));

        final unixPortFile = File(
          connect_impl.defaultPortFilePathForTest(
            'transport-alpha',
            transport: 'unix',
          ),
        );
        expect(unixPortFile.existsSync(), isTrue);
        expect(
          unixPortFile.readAsStringSync().trim(),
          equals(
            connect_impl.defaultUnixSocketURIForTest(
              'transport-alpha',
              unixPortFile.path,
            ),
          ),
        );
      },
    );
  });
}

bool _isWithinTempHierarchy(String path) {
  final socketDir = File(path).parent.absolute.path;
  var current = Directory.systemTemp.absolute.path;
  while (true) {
    if (socketDir == current) {
      return true;
    }
    final parent = Directory(current).parent.absolute.path;
    if (parent == current) {
      return false;
    }
    current = parent;
  }
}

String _shellQuote(String value) {
  return "'${value.replaceAll("'", "'\"'\"'")}'";
}
