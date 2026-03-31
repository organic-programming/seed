import 'dart:io';

import 'package:holons/holons.dart';
import 'package:holons/src/discover.dart' as discover_impl;
import 'package:test/test.dart';

import 'test_support/discovery_fixture.dart';

void main() {
  final sdkRoot = Directory.current.path;

  tearDown(discover_impl.resetDiscoveryTestOverrides);

  group('resolve', () {
    test('known slug', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
      );

      final result =
          resolve(LOCAL, 'alpha', fixture.root.path, CWD, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.ref?.info?.slug, equals('alpha'));
    });

    test('missing target', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      final result =
          resolve(LOCAL, 'missing', fixture.root.path, ALL, NO_TIMEOUT);

      expect(result.ref, isNull);
      expect(result.error, equals('holon "missing" not found'));
    });

    test('invalid specifiers', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      final result =
          resolve(LOCAL, 'alpha', fixture.root.path, 0xFF, NO_TIMEOUT);

      expect(result.ref, isNull);
      expect(result.error, contains('invalid specifiers'));
    });
  });
}
