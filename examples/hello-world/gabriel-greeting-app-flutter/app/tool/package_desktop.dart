import 'dart:convert';
import 'dart:ffi';
import 'dart:io';

import 'package:path/path.dart' as p;

const _slug = 'gabriel-greeting-app-flutter';
const _entryBase = 'gabriel_greeting_app_flutter';
const _givenName = 'Gabriel';
const _familyName = 'Greeting-App-Flutter';
const _motto = 'Flutter HostUI for the Gabriel greeting service.';
const _uuid = '2eb763d5-879c-43c1-8e23-9b87c9b4d2a6';
const _memberSlugs = <String>[
  'gabriel-greeting-go',
  'gabriel-greeting-swift',
  'gabriel-greeting-rust',
  'gabriel-greeting-python',
  'gabriel-greeting-c',
  'gabriel-greeting-cpp',
  'gabriel-greeting-csharp',
  'gabriel-greeting-dart',
  'gabriel-greeting-java',
  'gabriel-greeting-kotlin',
  'gabriel-greeting-node',
  'gabriel-greeting-ruby',
];
const _standaloneRunners = <String>{
  'go-module',
  'swift-package',
  'cargo',
  'cmake',
  'qt-cmake',
  'dart',
};

Future<void> main(List<String> args) async {
  if (args.length < 2 || args.length > 3) {
    stderr.writeln(
      'usage: dart run tool/package_desktop.dart <macos|windows|linux> <debug|release|profile> [normal|hardened]',
    );
    exitCode = 64;
    return;
  }

  final target = args[0].trim().toLowerCase();
  final mode = args[1].trim().toLowerCase();
  if (!{'macos', 'windows', 'linux'}.contains(target)) {
    stderr.writeln('unsupported target: $target');
    exitCode = 64;
    return;
  }
  if (!{'debug', 'release', 'profile'}.contains(mode)) {
    stderr.writeln('unsupported mode: $mode');
    exitCode = 64;
    return;
  }
  final hardened = _isHardenedBuild(args.length == 3 ? args[2] : null);

  final appDir = Directory.current;
  final rootDir = appDir.parent;
  final examplesDir = rootDir.parent;
  final packageDir = Directory(
    p.join(rootDir.path, '.op', 'build', '$_slug.holon'),
  );
  final runtimeArch = _runtimeArchitecture(target);
  final runtimeDir = Directory(p.join(packageDir.path, 'bin', runtimeArch));

  if (packageDir.existsSync()) {
    packageDir.deleteSync(recursive: true);
  }
  runtimeDir.createSync(recursive: true);

  late final String entrypoint;
  switch (target) {
    case 'macos':
      final source = _findMacOSAppBundle(
        Directory(
          p.join(
            appDir.path,
            'build',
            'macos',
            'Build',
            'Products',
            _macosMode(mode),
          ),
        ),
        preferredBundleName: '$_entryBase.app',
      );
      final destination = Directory(p.join(runtimeDir.path, '$_entryBase.app'));
      final entitlements = await _extractCodeSignEntitlements(source.path);
      _purgePackagedResources(source);
      _purgePackagedResources(destination);
      await _copyEntity(source, destination);
      await _copyMemberHolons(
        examplesDir,
        Directory(p.join(destination.path, 'Contents', 'Resources', 'Holons')),
      );
      await _copyAppProto(
        rootDir,
        Directory(
          p.join(destination.path, 'Contents', 'Resources', 'AppProto'),
        ),
      );
      final codesignArgs = <String>['--force', '--deep', '--sign', '-'];
      if (hardened && entitlements != null) {
        codesignArgs.addAll(<String>['--entitlements', entitlements.path]);
      }
      codesignArgs.add(destination.path);
      final codesignResult = await Process.run('codesign', codesignArgs);
      if (entitlements != null && entitlements.existsSync()) {
        entitlements.deleteSync();
      }
      if (codesignResult.exitCode != 0) {
        stderr.writeln('codesign warning: ${codesignResult.stderr}');
      }
      entrypoint = '$_entryBase.app';
      break;
    case 'linux':
      final bundle = _findBundleDirectory(
        Directory(p.join(appDir.path, 'build', 'linux')),
        '$_entryBase',
        requiredSidecarDir: 'data',
      );
      await _copyDirectoryContents(bundle, runtimeDir);
      await _copyMemberHolons(
        examplesDir,
        Directory(p.join(runtimeDir.path, 'data', 'Holons')),
      );
      await _copyAppProto(
        rootDir,
        Directory(p.join(runtimeDir.path, 'data', 'AppProto')),
      );
      entrypoint = _entryBase;
      break;
    case 'windows':
      final bundle = _findBundleDirectory(
        Directory(p.join(appDir.path, 'build', 'windows')),
        '$_entryBase.exe',
        requiredSidecarDir: 'data',
      );
      await _copyDirectoryContents(bundle, runtimeDir);
      await _copyMemberHolons(
        examplesDir,
        Directory(p.join(runtimeDir.path, 'data', 'Holons')),
      );
      await _copyAppProto(
        rootDir,
        Directory(p.join(runtimeDir.path, 'data', 'AppProto')),
      );
      entrypoint = '$_entryBase.exe';
      break;
  }

  await _writeHolonPackageJson(
    packageDir: packageDir,
    runtimeArchitecture: runtimeArch,
    entrypoint: entrypoint,
  );

  stdout.writeln(packageDir.path);
}

bool _isHardenedBuild(String? arg) {
  final normalizedArg = (arg ?? '').trim().toLowerCase();
  if (normalizedArg == 'hardened') {
    return true;
  }
  if (normalizedArg == 'normal') {
    return false;
  }

  final env = (Platform.environment['OP_BUILD_HARDENED'] ?? '')
      .trim()
      .toLowerCase();
  return env == '1' || env == 'true' || env == 'yes' || env == 'on';
}

String _macosMode(String mode) {
  switch (mode) {
    case 'release':
      return 'Release';
    case 'profile':
      return 'Profile';
    default:
      return 'Debug';
  }
}

String _runtimeArchitecture(String target) {
  final os = switch (target) {
    'macos' => 'darwin',
    'windows' => 'windows',
    'linux' => 'linux',
    _ => throw StateError('unsupported target: $target'),
  };

  final arch = switch (Abi.current()) {
    Abi.macosArm64 || Abi.linuxArm64 || Abi.windowsArm64 => 'arm64',
    Abi.macosX64 || Abi.linuxX64 || Abi.windowsX64 => 'amd64',
    _ => throw StateError('unsupported runtime ABI: ${Abi.current()}'),
  };
  return '${os}_$arch';
}

Directory _findBundleDirectory(
  Directory root,
  String executableName, {
  required String requiredSidecarDir,
}) {
  if (!root.existsSync()) {
    throw StateError('missing build directory: ${root.path}');
  }

  final matches = <Directory>[];
  for (final entity in root.listSync(recursive: true, followLinks: false)) {
    if (entity is! File || p.basename(entity.path) != executableName) {
      continue;
    }
    final candidate = entity.parent;
    if (Directory(p.join(candidate.path, requiredSidecarDir)).existsSync()) {
      matches.add(candidate);
    }
  }

  if (matches.isEmpty) {
    throw StateError(
      'could not locate bundle for $executableName under ${root.path}',
    );
  }

  matches.sort((left, right) => left.path.length.compareTo(right.path.length));
  return matches.first;
}

Directory _findMacOSAppBundle(
  Directory root, {
  required String preferredBundleName,
}) {
  if (!root.existsSync()) {
    throw StateError('missing macOS products directory: ${root.path}');
  }

  final preferred = Directory(p.join(root.path, preferredBundleName));
  if (preferred.existsSync()) {
    return preferred;
  }

  final bundles = root
      .listSync(followLinks: false)
      .whereType<Directory>()
      .where((directory) => p.extension(directory.path) == '.app')
      .toList(growable: false);
  if (bundles.isEmpty) {
    throw StateError('missing macOS app bundle under ${root.path}');
  }
  if (bundles.length == 1) {
    return bundles.first;
  }

  throw StateError(
    'multiple macOS app bundles found under ${root.path}: '
    '${bundles.map((bundle) => p.basename(bundle.path)).join(', ')}',
  );
}

Future<void> _copyMemberHolons(
  Directory examplesDir,
  Directory destination,
) async {
  destination.createSync(recursive: true);
  for (final slug in _memberSlugs) {
    final source = Directory(
      p.join(examplesDir.path, slug, '.op', 'build', '$slug.holon'),
    );
    if (!source.existsSync()) {
      if (_memberProducesStandaloneArtifact(examplesDir, slug)) {
        throw StateError('missing built member package: ${source.path}');
      }
      stderr.writeln(
        'skipping missing non-standalone member package: ${source.path}',
      );
      continue;
    }
    await _copyEntity(
      source,
      Directory(p.join(destination.path, '$slug.holon')),
    );
  }
}

Future<void> _copyAppProto(Directory rootDir, Directory destination) async {
  final source = Directory(p.join(rootDir.path, 'api'));
  if (!source.existsSync()) {
    throw StateError('missing app proto directory: ${source.path}');
  }
  await _copyEntity(source, destination);
}

void _purgePackagedResources(Directory appBundle) {
  if (!appBundle.existsSync()) {
    return;
  }
  for (final relativePath in <String>[
    p.join('Contents', 'Resources', 'Holons'),
    p.join('Contents', 'Resources', 'AppProto'),
  ]) {
    final path = p.join(appBundle.path, relativePath);
    final directory = Directory(path);
    if (directory.existsSync()) {
      directory.deleteSync(recursive: true);
    }
  }
}

Future<void> _writeHolonPackageJson({
  required Directory packageDir,
  required String runtimeArchitecture,
  required String entrypoint,
}) async {
  final payload = <String, Object?>{
    'schema': 'holon-package/v1',
    'slug': _slug,
    'uuid': _uuid,
    'identity': <String, Object?>{
      'given_name': _givenName,
      'family_name': _familyName,
      'motto': _motto,
    },
    'lang': 'dart',
    'runner': 'recipe',
    'status': 'draft',
    'kind': 'composite',
    'transport': 'stdio',
    'entrypoint': entrypoint,
    'architectures': <String>[runtimeArchitecture],
    'standalone': true,
    'has_dist': false,
    'has_source': false,
  };

  final file = File(p.join(packageDir.path, '.holon.json'));
  file.parent.createSync(recursive: true);
  await file.writeAsString(
    const JsonEncoder.withIndent('  ').convert(payload) + '\n',
  );
}

Future<void> _copyDirectoryContents(
  Directory source,
  Directory destination,
) async {
  destination.createSync(recursive: true);
  for (final entity in source.listSync(followLinks: false)) {
    final name = p.basename(entity.path);
    await _copyEntity(entity, Directory(p.join(destination.path, name)));
  }
}

bool _memberProducesStandaloneArtifact(Directory examplesDir, String slug) {
  final packageMetadata = File(
    p.join(
      examplesDir.path,
      slug,
      '.op',
      'build',
      '$slug.holon',
      '.holon.json',
    ),
  );
  if (packageMetadata.existsSync()) {
    final decoded = jsonDecode(packageMetadata.readAsStringSync());
    if (decoded is Map<String, dynamic>) {
      final standalone = decoded['standalone'];
      if (standalone is bool) {
        return standalone;
      }
    }
  }

  final manifest = File(
    p.join(examplesDir.path, slug, 'api', 'v1', 'holon.proto'),
  );
  if (!manifest.existsSync()) {
    return true;
  }

  final runnerMatch = RegExp(
    r'runner:\s*"([^"]+)"',
  ).firstMatch(manifest.readAsStringSync());
  final runner = runnerMatch?.group(1)?.trim().toLowerCase() ?? '';
  return _standaloneRunners.contains(runner);
}

Future<void> _copyEntity(
  FileSystemEntity source,
  FileSystemEntity destination,
) async {
  final type = FileSystemEntity.typeSync(source.path, followLinks: false);
  switch (type) {
    case FileSystemEntityType.directory:
      final srcDir = Directory(source.path);
      final dstDir = Directory(destination.path);
      if (dstDir.existsSync()) {
        dstDir.deleteSync(recursive: true);
      }
      dstDir.createSync(recursive: true);
      for (final child in srcDir.listSync(followLinks: false)) {
        final name = p.basename(child.path);
        await _copyEntity(child, Directory(p.join(dstDir.path, name)));
      }
      break;
    case FileSystemEntityType.file:
      final dstFile = File(destination.path);
      dstFile.parent.createSync(recursive: true);
      await File(source.path).copy(dstFile.path);
      await _copyModeIfNeeded(source.path, dstFile.path);
      break;
    case FileSystemEntityType.link:
      final dstLink = Link(destination.path);
      dstLink.parent.createSync(recursive: true);
      if (dstLink.existsSync()) {
        dstLink.deleteSync();
      }
      final target = await Link(source.path).target();
      await dstLink.create(target);
      break;
    case FileSystemEntityType.notFound:
      throw StateError('missing source entity: ${source.path}');
  }
}

Future<void> _copyModeIfNeeded(
  String sourcePath,
  String destinationPath,
) async {
  if (Platform.isWindows) {
    return;
  }
  final mode = FileStat.statSync(sourcePath).mode & 0x1FF;
  final octal = mode.toRadixString(8).padLeft(3, '0');
  final result = await Process.run('chmod', <String>[octal, destinationPath]);
  if (result.exitCode != 0) {
    throw StateError(
      'failed to preserve mode on $destinationPath: ${result.stderr}',
    );
  }
}

Future<File?> _extractCodeSignEntitlements(String appPath) async {
  final result = await Process.run('codesign', <String>[
    '-d',
    '--entitlements',
    ':-',
    appPath,
  ]);
  if (result.exitCode != 0) {
    stderr.writeln(
      'codesign warning: failed to read entitlements from $appPath: ${result.stderr}',
    );
    return null;
  }

  final stdoutText = result.stdout is String
      ? result.stdout as String
      : utf8.decode(result.stdout as List<int>);
  final start = stdoutText.indexOf('<?xml');
  if (start < 0) {
    return null;
  }
  final plist = stdoutText.substring(start).trim();
  if (plist.isEmpty) {
    return null;
  }

  final file = File(
    p.join(
      Directory.systemTemp.path,
      '$_slug-entitlements-${DateTime.now().microsecondsSinceEpoch}.plist',
    ),
  );
  await file.writeAsString('$plist\n');
  return file;
}
