import 'dart:convert';
import 'dart:io';

import 'package:holons/holons.dart';
import 'package:test/test.dart';

void main() {
  final sdkRoot = Directory.current.path;

  group('discover', () {
    test('recurses skips and dedups by uuid', () async {
      final root = Directory.systemTemp.createTempSync('holons_discover_dart_');
      addTearDown(() => root.delete(recursive: true));

      _writeHolon(root.path, 'holons/alpha',
          const _HolonSeed('uuid-alpha', 'Alpha', 'Go', 'alpha-go'));
      _writeHolon(root.path, 'nested/beta',
          const _HolonSeed('uuid-beta', 'Beta', 'Rust', 'beta-rust'));
      _writeHolon(root.path, 'nested/dup/alpha',
          const _HolonSeed('uuid-alpha', 'Alpha', 'Go', 'alpha-go'));

      for (final skipped in <String>[
        '.git/hidden',
        '.op/hidden',
        'node_modules/hidden',
        'vendor/hidden',
        'build/hidden',
        '.cache/hidden',
      ]) {
        _writeHolon(
            root.path,
            skipped,
            const _HolonSeed(
                'ignored-uuid', 'Ignored', 'Holon', 'ignored-holon'));
      }

      final entries = await discover(root.path);
      expect(entries, hasLength(2));

      final alpha = entries.firstWhere((entry) => entry.uuid == 'uuid-alpha');
      expect(alpha.slug, equals('alpha-go'));
      expect(alpha.relativePath, equals('holons/alpha'));
      expect(alpha.manifest?.build.runner, equals('go-module'));

      final beta = entries.firstWhere((entry) => entry.uuid == 'uuid-beta');
      expect(beta.relativePath, equals('nested/beta'));
    });

    test('discoverLocal and find helpers use the current directory', () async {
      final root = Directory.systemTemp.createTempSync('holons_find_dart_');
      addTearDown(() => root.delete(recursive: true));

      _writeHolon(
        root.path,
        'rob-go',
        const _HolonSeed(
          'c7f3a1b2-1111-1111-1111-111111111111',
          'Rob',
          'Go',
          'rob-go',
        ),
      );

      final runner = _writeDiscoverRunnerScript(sdkRoot);
      addTearDown(() {
        if (runner.existsSync()) {
          runner.deleteSync();
        }
      });

      final result = await Process.run(
        Platform.resolvedExecutable,
        <String>[runner.path],
        workingDirectory: root.path,
      );
      expect(
        result.exitCode,
        equals(0),
        reason:
            'discover runner failed:\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}',
      );

      final decoded =
          jsonDecode((result.stdout as String).trim()) as Map<String, dynamic>;
      expect(decoded['localLength'], equals(1));
      expect(decoded['localSlug'], equals('rob-go'));
      expect(
        decoded['bySlugUuid'],
        equals('c7f3a1b2-1111-1111-1111-111111111111'),
      );
      expect(decoded['byUuidSlug'], equals('rob-go'));
      expect(decoded['missing'], isNull);
    });

    test('skips unreadable directories', () async {
      if (Platform.isWindows) {
        return;
      }

      final root = Directory.systemTemp.createTempSync('holons_unreadable_');
      final locked = Directory('${root.path}/locked')..createSync();
      addTearDown(() {
        Process.runSync('chmod', <String>['755', locked.path]);
        root.deleteSync(recursive: true);
      });

      _writeHolon(
        root.path,
        'readable',
        const _HolonSeed(
          '3d7fe412-8f34-44d7-8ef2-b222f25c1dbb',
          'Readable',
          'Go',
          'readable-go',
        ),
      );
      _writeHolon(
        root.path,
        'locked/hidden',
        const _HolonSeed(
          'd4f62503-c6e2-4874-a607-e58d0c993f68',
          'Hidden',
          'Go',
          'hidden-go',
        ),
      );

      final chmod = Process.runSync('chmod', <String>['000', locked.path]);
      expect(
        chmod.exitCode,
        equals(0),
        reason: 'chmod failed: ${chmod.stderr}',
      );

      final entries = await discover(root.path);
      expect(entries, hasLength(1));
      expect(entries.single.slug, equals('readable-go'));
    });
  });
}

class _HolonSeed {
  final String uuid;
  final String givenName;
  final String familyName;
  final String binary;

  const _HolonSeed(this.uuid, this.givenName, this.familyName, this.binary);
}

void _writeHolon(String root, String relativeDir, _HolonSeed seed) {
  final dir = Directory('$root/$relativeDir')..createSync(recursive: true);
  final file = File('${dir.path}/holon.proto');
  file.writeAsStringSync('''
syntax = "proto3";

package test.v1;

option (holons.v1.manifest) = {
  identity: {
    uuid: "${seed.uuid}"
    given_name: "${seed.givenName}"
    family_name: "${seed.familyName}"
    motto: "Test"
    composer: "test"
    clade: "deterministic/pure"
    status: "draft"
    born: "2026-03-07"
  }
  lineage: {
    generated_by: "test"
  }
  kind: "native"
  build: {
    runner: "go-module"
  }
  artifacts: {
    binary: "${seed.binary}"
  }
};
''');
}

File _writeDiscoverRunnerScript(String sdkRoot) {
  final runner = File(
    '$sdkRoot/.dart_tool/discover-runner-${DateTime.now().microsecondsSinceEpoch}.dart',
  );
  runner.parent.createSync(recursive: true);
  runner.writeAsStringSync('''
import 'dart:convert';

import 'package:holons/holons.dart';

Future<void> main() async {
  final local = await discoverLocal();
  final bySlug = await findBySlug('rob-go');
  final byUuid = await findByUUID('c7f3a1b2');
  final missing = await findBySlug('missing');

  print(jsonEncode(<String, Object?>{
    'localLength': local.length,
    'localSlug': local.single.slug,
    'bySlugUuid': bySlug?.uuid,
    'byUuidSlug': byUuid?.slug,
    'missing': missing?.slug,
  }));
}
''');
  return runner;
}
