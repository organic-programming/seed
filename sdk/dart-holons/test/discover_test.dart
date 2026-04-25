import 'dart:io';

import 'package:holons/holons.dart';
import 'package:holons/src/discover.dart' as discover_impl;
import 'package:test/test.dart';

import 'test_support/discovery_fixture.dart';

void main() {
  final sdkRoot = Directory.current.path;

  tearDown(discover_impl.resetDiscoveryTestOverrides);

  group('Discover', () {
    test('default siblings root finds macOS app resources', () {
      final sandbox = Directory.systemTemp.createTempSync(
        'dart-holons-published-app-',
      );
      addTearDown(() => sandbox.deleteSync(recursive: true));

      final executableDir = Directory(
        '${sandbox.path}/Gabriel.app/Contents/MacOS',
      )..createSync(recursive: true);
      final holonsDir = Directory(
        '${sandbox.path}/Gabriel.app/Contents/Resources/Holons',
      )..createSync(recursive: true);

      discover_impl.discoverResolvedExecutableProvider =
          () => '${executableDir.path}/gabriel';

      expect(
        discover_impl.discoverSiblingsRootProvider(),
        equals(holonsDir.path),
      );
    });

    test('default siblings root finds packaged data holons', () {
      final sandbox = Directory.systemTemp.createTempSync(
        'dart-holons-published-data-',
      );
      addTearDown(() => sandbox.deleteSync(recursive: true));

      final runtimeDir = Directory('${sandbox.path}/package/bin/darwin_arm64')
        ..createSync(recursive: true);
      final holonsDir = Directory('${runtimeDir.path}/data/Holons')
        ..createSync(recursive: true);

      discover_impl.discoverResolvedExecutableProvider =
          () => '${runtimeDir.path}/gabriel';

      expect(
        discover_impl.discoverSiblingsRootProvider(),
        equals(holonsDir.path),
      );
    });

    test('discover all layers', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));

      configureDiscoveryRuntime(
        fixture,
        siblingsRootProvider: () => fixture.siblings.path,
        sourceBridge: (scope, expression, root, specifiers, limit, timeout) {
          return DiscoverResult(
            found: <HolonRef>[
              HolonRef(
                url: Uri.file('${fixture.root.path}/source/source-alpha')
                    .toString(),
                info: const HolonInfo(
                  slug: 'source-alpha',
                  uuid: 'uuid-source-alpha',
                  identity:
                      IdentityInfo(givenName: 'Source', familyName: 'Alpha'),
                  hasSource: true,
                ),
              ),
            ],
          );
        },
      );

      writePackageHolon(
        Directory('${fixture.siblings.path}/siblings-alpha.holon'),
        slug: 'siblings-alpha',
        uuid: 'uuid-siblings-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/cwd-beta.holon'),
        slug: 'cwd-beta',
        uuid: 'uuid-cwd-beta',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/.op/build/built-gamma.holon'),
        slug: 'built-gamma',
        uuid: 'uuid-built-gamma',
      );
      writePackageHolon(
        Directory('${fixture.opBin.path}/installed-delta.holon'),
        slug: 'installed-delta',
        uuid: 'uuid-installed-delta',
      );
      writePackageHolon(
        Directory('${fixture.cache.path}/deps/cached-epsilon.holon'),
        slug: 'cached-epsilon',
        uuid: 'uuid-cached-epsilon',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, ALL, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(
        slugs(result),
        equals(<String>[
          'siblings-alpha',
          'cwd-beta',
          'source-alpha',
          'built-gamma',
          'installed-delta',
          'cached-epsilon',
        ]),
      );
    });

    test('published macOS app discovery uses only bundled holons', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture, currentRoot: fixture.root.path);

      final sandbox = Directory.systemTemp.createTempSync(
        'dart-holons-published-macos-all-',
      );
      addTearDown(() => sandbox.deleteSync(recursive: true));

      final executableDir = Directory(
        '${sandbox.path}/Gabriel.app/Contents/MacOS',
      )..createSync(recursive: true);
      final holonsDir = Directory(
        '${sandbox.path}/Gabriel.app/Contents/Resources/Holons',
      )..createSync(recursive: true);

      discover_impl.discoverResolvedExecutableProvider =
          () => '${executableDir.path}/gabriel';

      writePackageHolon(
        Directory('${holonsDir.path}/published-alpha.holon'),
        slug: 'published-alpha',
        uuid: 'uuid-published-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/cwd-beta.holon'),
        slug: 'cwd-beta',
        uuid: 'uuid-cwd-beta',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/.op/build/built-gamma.holon'),
        slug: 'built-gamma',
        uuid: 'uuid-built-gamma',
      );
      writePackageHolon(
        Directory('${fixture.opBin.path}/installed-delta.holon'),
        slug: 'installed-delta',
        uuid: 'uuid-installed-delta',
      );
      writePackageHolon(
        Directory('${fixture.cache.path}/deps/cached-epsilon.holon'),
        slug: 'cached-epsilon',
        uuid: 'uuid-cached-epsilon',
      );

      final result = Discover(LOCAL, null, null, ALL, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['published-alpha']));
    });

    test('published Linux/Windows-style discovery uses only data holons', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture, currentRoot: fixture.root.path);

      final sandbox = Directory.systemTemp.createTempSync(
        'dart-holons-published-data-all-',
      );
      addTearDown(() => sandbox.deleteSync(recursive: true));

      final runtimeDir = Directory('${sandbox.path}/package/bin/linux_x64')
        ..createSync(recursive: true);
      final holonsDir = Directory('${sandbox.path}/package/data/Holons')
        ..createSync(recursive: true);

      discover_impl.discoverResolvedExecutableProvider =
          () => '${runtimeDir.path}/gabriel';

      writePackageHolon(
        Directory('${holonsDir.path}/published-alpha.holon'),
        slug: 'published-alpha',
        uuid: 'uuid-published-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/cwd-beta.holon'),
        slug: 'cwd-beta',
        uuid: 'uuid-cwd-beta',
      );
      writePackageHolon(
        Directory('${fixture.opBin.path}/installed-delta.holon'),
        slug: 'installed-delta',
        uuid: 'uuid-installed-delta',
      );

      final result = Discover(LOCAL, null, null, ALL, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['published-alpha']));
    });

    test('published discovery still resolves bundled slug lookups', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture, currentRoot: fixture.root.path);

      final sandbox = Directory.systemTemp.createTempSync(
        'dart-holons-published-slug-',
      );
      addTearDown(() => sandbox.deleteSync(recursive: true));

      final executableDir = Directory(
        '${sandbox.path}/Gabriel.app/Contents/MacOS',
      )..createSync(recursive: true);
      final holonsDir = Directory(
        '${sandbox.path}/Gabriel.app/Contents/Resources/Holons',
      )..createSync(recursive: true);

      discover_impl.discoverResolvedExecutableProvider =
          () => '${executableDir.path}/gabriel';

      writePackageHolon(
        Directory('${holonsDir.path}/published-alpha.holon'),
        slug: 'published-alpha',
        uuid: 'uuid-published-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/cwd-beta.holon'),
        slug: 'cwd-beta',
        uuid: 'uuid-cwd-beta',
      );

      final result =
          Discover(LOCAL, 'published-alpha', null, ALL, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['published-alpha']));
    });

    test('explicit root still uses source-mode discovery even in published app',
        () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture, currentRoot: fixture.root.path);

      final sandbox = Directory.systemTemp.createTempSync(
        'dart-holons-published-explicit-root-',
      );
      addTearDown(() => sandbox.deleteSync(recursive: true));

      final executableDir = Directory(
        '${sandbox.path}/Gabriel.app/Contents/MacOS',
      )..createSync(recursive: true);
      final holonsDir = Directory(
        '${sandbox.path}/Gabriel.app/Contents/Resources/Holons',
      )..createSync(recursive: true);

      discover_impl.discoverResolvedExecutableProvider =
          () => '${executableDir.path}/gabriel';

      writePackageHolon(
        Directory('${holonsDir.path}/published-alpha.holon'),
        slug: 'published-alpha',
        uuid: 'uuid-published-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/cwd-beta.holon'),
        slug: 'cwd-beta',
        uuid: 'uuid-cwd-beta',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['cwd-beta']));
    });

    test('filter by specifiers', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/cwd-alpha.holon'),
        slug: 'cwd-alpha',
        uuid: 'uuid-cwd-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/.op/build/built-beta.holon'),
        slug: 'built-beta',
        uuid: 'uuid-built-beta',
      );
      writePackageHolon(
        Directory('${fixture.opBin.path}/installed-gamma.holon'),
        slug: 'installed-gamma',
        uuid: 'uuid-installed-gamma',
      );

      final result = Discover(LOCAL, null, fixture.root.path, BUILT | INSTALLED,
          NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['built-beta', 'installed-gamma']));
    });

    test('match by slug', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/beta.holon'),
        slug: 'beta',
        uuid: 'uuid-beta',
      );

      final result =
          Discover(LOCAL, 'beta', fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['beta']));
    });

    test('match by alias', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
        aliases: const <String>['first'],
      );

      final result = Discover(
          LOCAL, 'first', fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['alpha']));
    });

    test('match by UUID prefix', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: '12345678-aaaa',
      );

      final result = Discover(
          LOCAL, '12345678', fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['alpha']));
    });

    test('match by path', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      final packageDir = Directory('${fixture.root.path}/path-alpha.holon');
      writePackageHolon(
        packageDir,
        slug: 'path-alpha',
        uuid: 'uuid-path-alpha',
      );

      final result = Discover(
          LOCAL, packageDir.path, fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['path-alpha']));
    });

    test('limit one', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/beta.holon'),
        slug: 'beta',
        uuid: 'uuid-beta',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, 1, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.found, hasLength(1));
    });

    test('limit zero means unlimited', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/beta.holon'),
        slug: 'beta',
        uuid: 'uuid-beta',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, 0, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.found, hasLength(2));
    });

    test('negative limit returns empty', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, -1, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.found, isEmpty);
    });

    test('invalid specifiers', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      final result =
          Discover(LOCAL, null, fixture.root.path, 0xFF, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, contains('invalid specifiers'));
    });

    test('specifiers zero treated as all', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/cwd-alpha.holon'),
        slug: 'cwd-alpha',
        uuid: 'uuid-cwd-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/.op/build/built-beta.holon'),
        slug: 'built-beta',
        uuid: 'uuid-built-beta',
      );
      writePackageHolon(
        Directory('${fixture.opBin.path}/installed-gamma.holon'),
        slug: 'installed-gamma',
        uuid: 'uuid-installed-gamma',
      );
      writePackageHolon(
        Directory('${fixture.cache.path}/deps/cached-delta.holon'),
        slug: 'cached-delta',
        uuid: 'uuid-cached-delta',
      );

      final allResult =
          Discover(LOCAL, null, fixture.root.path, ALL, NO_LIMIT, NO_TIMEOUT);
      final zeroResult =
          Discover(LOCAL, null, fixture.root.path, 0, NO_LIMIT, NO_TIMEOUT);

      expect(zeroResult.error, isNull);
      expect(slugs(zeroResult), equals(slugs(allResult)));
    });

    test('null expression returns all', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/beta.holon'),
        slug: 'beta',
        uuid: 'uuid-beta',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.found, hasLength(2));
    });

    test('missing expression returns empty', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
      );

      final result = Discover(
          LOCAL, 'missing', fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.found, isEmpty);
    });

    test('excluded dirs skipped', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/kept.holon'),
        slug: 'kept',
        uuid: 'uuid-kept',
      );
      for (final skipped in <String>[
        '.git/hidden.holon',
        '.op/hidden.holon',
        'node_modules/hidden.holon',
        'vendor/hidden.holon',
        'build/hidden.holon',
        'testdata/hidden.holon',
        '.cache/hidden.holon',
      ]) {
        writePackageHolon(
          Directory('${fixture.root.path}/$skipped'),
          slug: 'hidden',
          uuid: 'uuid-$skipped',
        );
      }

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['kept']));
    });

    test('deduplicate by UUID', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'same-uuid',
      );
      writePackageHolon(
        Directory('${fixture.root.path}/nested/alpha-copy.holon'),
        slug: 'alpha-copy',
        uuid: 'same-uuid',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.found, hasLength(1));
      expect(result.found.single.info?.slug, equals('alpha'));
    });

    test('`.holon.json` fast path', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/fast-path.holon'),
        slug: 'fast-path',
        uuid: 'uuid-fast-path',
        runner: 'dart',
        entrypoint: 'fast-binary',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.found.single.info?.runner, equals('dart'));
      expect(result.found.single.info?.entrypoint, equals('fast-binary'));
    });

    test('missing package metadata does not launch packaged app executable',
        () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      final executableDir = Directory(
        '${fixture.sandbox.path}/Gabriel.app/Contents/MacOS',
      )..createSync(recursive: true);
      discover_impl.discoverResolvedExecutableProvider =
          () => '${executableDir.path}/Gabriel';
      Directory('${fixture.root.path}/missing-metadata.holon')
          .createSync(recursive: true);

      final result =
          Discover(LOCAL, null, fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(result.found, hasLength(1));
      expect(result.found.single.error, contains('requires the Dart CLI'));
    });

    test(
      'Describe fallback when `.holon.json` is missing',
      () {
        final helper = buildDescribeHelper(sdkRoot);
        if (helper == null) {
          return;
        }
        final fixture = createRuntimeFixture(sdkRoot);
        addTearDown(() => deleteFixture(fixture));
        configureDiscoveryRuntime(fixture);

        writeDescribePackageHolon(
          Directory('${fixture.root.path}/fallback.holon'),
          executablePath: helper,
          slug: 'fallback',
          entrypoint: 'fallback',
        );

        final result =
            Discover(LOCAL, null, fixture.root.path, CWD, NO_LIMIT, NO_TIMEOUT);

        expect(result.error, isNull);
        expect(result.found.single.info?.slug, equals('fallback'));
      },
      skip: Platform.isWindows
          ? 'fixture binary uses a POSIX shell launcher'
          : false,
    );

    test('siblings layer', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(
        fixture,
        siblingsRootProvider: () => fixture.siblings.path,
      );

      writePackageHolon(
        Directory('${fixture.siblings.path}/siblings-alpha.holon'),
        slug: 'siblings-alpha',
        uuid: 'uuid-siblings-alpha',
      );

      final result = Discover(
          LOCAL, null, fixture.root.path, SIBLINGS, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['siblings-alpha']));
    });

    test('source layer offloads to local `op`', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));

      late List<Object?> captured;
      configureDiscoveryRuntime(
        fixture,
        sourceBridge: (scope, expression, root, specifiers, limit, timeout) {
          captured = <Object?>[
            scope,
            expression,
            root,
            specifiers,
            limit,
            timeout
          ];
          return DiscoverResult(
            found: <HolonRef>[
              HolonRef(
                url: Uri.file('${fixture.root.path}/source-alpha').toString(),
                info: const HolonInfo(
                  slug: 'source-alpha',
                  uuid: 'uuid-source-alpha',
                  identity:
                      IdentityInfo(givenName: 'Source', familyName: 'Alpha'),
                  hasSource: true,
                ),
              ),
            ],
          );
        },
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, SOURCE, NO_LIMIT, 5000);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['source-alpha']));
      expect(
        captured,
        equals(
            <Object?>[LOCAL, null, fixture.root.path, SOURCE, NO_LIMIT, 5000]),
      );
    });

    test('built layer', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.root.path}/.op/build/built-alpha.holon'),
        slug: 'built-alpha',
        uuid: 'uuid-built-alpha',
      );

      final result =
          Discover(LOCAL, null, fixture.root.path, BUILT, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['built-alpha']));
    });

    test('installed layer', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.opBin.path}/installed-alpha.holon'),
        slug: 'installed-alpha',
        uuid: 'uuid-installed-alpha',
      );

      final result = Discover(
          LOCAL, null, fixture.root.path, INSTALLED, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['installed-alpha']));
    });

    test('cached layer', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      writePackageHolon(
        Directory('${fixture.cache.path}/deps/cached-alpha.holon'),
        slug: 'cached-alpha',
        uuid: 'uuid-cached-alpha',
      );

      final result = Discover(
          LOCAL, null, fixture.root.path, CACHED, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['cached-alpha']));
    });

    test('nil root defaults to cwd', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture, currentRoot: fixture.root.path);

      writePackageHolon(
        Directory('${fixture.root.path}/alpha.holon'),
        slug: 'alpha',
        uuid: 'uuid-alpha',
      );

      final result = Discover(LOCAL, null, null, CWD, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, isNull);
      expect(slugs(result), equals(<String>['alpha']));
    });

    test('empty root returns error', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      final result = Discover(LOCAL, null, '', ALL, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, equals('root cannot be empty'));
    });

    test('unsupported scope returns error', () {
      final fixture = createRuntimeFixture(sdkRoot);
      addTearDown(() => deleteFixture(fixture));
      configureDiscoveryRuntime(fixture);

      final result =
          Discover(PROXY, null, fixture.root.path, ALL, NO_LIMIT, NO_TIMEOUT);

      expect(result.error, equals('scope 1 not supported'));
    });
  });
}
