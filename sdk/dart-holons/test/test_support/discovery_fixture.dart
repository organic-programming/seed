import 'dart:convert';
import 'dart:io';

import 'package:holons/holons.dart';
import 'package:holons/src/discover.dart' as discover_impl;
import 'package:holons/src/discovery_probe.dart';

class RuntimeFixture {
  final Directory sandbox;
  final Directory root;
  final Directory opHome;
  final Directory opBin;
  final Directory cache;
  final Directory siblings;
  final String sdkRoot;

  RuntimeFixture({
    required this.sandbox,
    required this.root,
    required this.opHome,
    required this.opBin,
    required this.cache,
    required this.siblings,
    required this.sdkRoot,
  });
}

String? _cachedDescribeHelperPath;

RuntimeFixture createRuntimeFixture(String sdkRoot) {
  final sandbox = Directory.systemTemp.createTempSync('dart-holons-discovery-');
  final root = Directory('${sandbox.path}/root')..createSync(recursive: true);
  final opHome = Directory('${sandbox.path}/runtime/.op')
    ..createSync(recursive: true);
  final opBin = Directory('${sandbox.path}/runtime/bin')
    ..createSync(recursive: true);
  final cache = Directory('${opHome.path}/cache')..createSync(recursive: true);
  final siblings = Directory('${sandbox.path}/siblings')
    ..createSync(recursive: true);

  return RuntimeFixture(
    sandbox: sandbox,
    root: root,
    opHome: opHome,
    opBin: opBin,
    cache: cache,
    siblings: siblings,
    sdkRoot: sdkRoot,
  );
}

void configureDiscoveryRuntime(
  RuntimeFixture fixture, {
  String? currentRoot,
  String? Function()? siblingsRootProvider,
  discover_impl.SourceDiscoverBridge? sourceBridge,
}) {
  discover_impl.resetDiscoveryTestOverrides();
  discover_impl.discoverCurrentRootProvider =
      () => currentRoot ?? fixture.root.path;
  discover_impl.discoverEnvironmentProvider = () => <String, String>{
        'HOME': fixture.sandbox.path,
        'OPPATH': fixture.opHome.path,
        'OPBIN': fixture.opBin.path,
      };
  discover_impl.discoverSiblingsRootProvider =
      siblingsRootProvider ?? () => null;
  discover_impl.discoverDartRunWorkingDirectoryProvider = () => fixture.sdkRoot;
  if (sourceBridge != null) {
    discover_impl.discoverSourceBridge = sourceBridge;
  }
}

void writePackageHolon(
  Directory dir, {
  required String slug,
  required String uuid,
  String? givenName,
  String? familyName,
  String lang = 'dart',
  String runner = 'dart',
  String status = 'draft',
  String kind = 'native',
  String transport = '',
  String entrypoint = 'fixture-server',
  List<String> aliases = const <String>[],
  List<String> architectures = const <String>[],
  bool hasDist = false,
  bool hasSource = false,
}) {
  dir.createSync(recursive: true);
  final names = _splitSlug(slug, givenName: givenName, familyName: familyName);
  final payload = <String, Object?>{
    'schema': 'holon-package/v1',
    'slug': slug,
    'uuid': uuid,
    'identity': <String, Object?>{
      'given_name': names.$1,
      'family_name': names.$2,
      'motto': 'Fixture',
      'aliases': aliases,
    },
    'lang': lang,
    'runner': runner,
    'status': status,
    'kind': kind,
    'transport': transport,
    'entrypoint': entrypoint,
    'architectures': architectures,
    'has_dist': hasDist,
    'has_source': hasSource,
  };
  File('${dir.path}/.holon.json').writeAsStringSync(
      '${const JsonEncoder.withIndent('  ').convert(payload)}\n');
}

void writeDescribePackageHolon(
  Directory dir, {
  required String executablePath,
  required String slug,
  String entrypoint = 'fixture-server',
}) {
  dir.createSync(recursive: true);
  final archDir = Directory('${dir.path}/bin/${currentArchDirectory()}')
    ..createSync(recursive: true);
  final launcher = File('${archDir.path}/$entrypoint');
  launcher.writeAsStringSync('''
#!/bin/sh
exec ${_shellQuote(executablePath)} --slug ${_shellQuote(slug)} "\$@"
''');
  if (!Platform.isWindows) {
    Process.runSync('chmod', <String>['755', launcher.path]);
  }
}

List<String> slugs(DiscoverResult result) {
  return result.found.map((ref) => ref.info?.slug ?? '').toList();
}

void deleteFixture(RuntimeFixture fixture) {
  if (fixture.sandbox.existsSync()) {
    fixture.sandbox.deleteSync(recursive: true);
  }
}

String? buildDescribeHelper(String sdkRoot) {
  final cached = _cachedDescribeHelperPath;
  if (cached != null && File(cached).existsSync()) {
    return cached;
  }

  final goHolonsDir = '$sdkRoot/../go-holons';
  final sourcePath =
      '$sdkRoot/../swift-holons/Tests/HolonsTests/Fixtures/connect-helper-go/main.go';
  if (!Directory(goHolonsDir).existsSync() || !File(sourcePath).existsSync()) {
    return null;
  }

  final outputPath = '${Directory.systemTemp.path}/dart-holons-describe-helper';
  final result = Process.runSync(
    _resolveGoBinary(),
    <String>['build', '-o', outputPath, sourcePath],
    workingDirectory: goHolonsDir,
    environment: _withGoCache(),
  );
  if (result.exitCode != 0) {
    return null;
  }

  _cachedDescribeHelperPath = outputPath;
  return outputPath;
}

(String, String) _splitSlug(
  String slug, {
  String? givenName,
  String? familyName,
}) {
  if (givenName != null && familyName != null) {
    return (givenName, familyName);
  }
  final parts = slug
      .split('-')
      .where((part) => part.trim().isNotEmpty)
      .toList(growable: false);
  final left =
      givenName ?? (parts.isNotEmpty ? _titleCase(parts.first) : 'Fixture');
  final right = familyName ??
      (parts.length > 1 ? parts.skip(1).map(_titleCase).join('-') : 'Holon');
  return (left, right);
}

String _titleCase(String value) {
  if (value.isEmpty) {
    return value;
  }
  return value[0].toUpperCase() + value.substring(1);
}

String _shellQuote(String value) {
  return "'${value.replaceAll("'", "'\"'\"'")}'";
}

String _resolveGoBinary() {
  final fromEnv = (Platform.environment['GO_BIN'] ?? '').trim();
  if (fromEnv.isNotEmpty) {
    return fromEnv;
  }

  const preferredGoBinary = '/Users/bpds/go/go1.25.1/bin/go';
  final preferred = File(preferredGoBinary);
  if (preferred.existsSync()) {
    return preferred.path;
  }

  return 'go';
}

Map<String, String> _withGoCache() {
  final environment = Map<String, String>.from(Platform.environment);
  if ((environment['GOCACHE'] ?? '').trim().isEmpty) {
    environment['GOCACHE'] = '${Directory.systemTemp.path}/go-cache';
  }
  return environment;
}
