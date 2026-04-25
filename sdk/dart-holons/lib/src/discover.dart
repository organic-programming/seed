import 'dart:convert';
import 'dart:io';

import 'discovery_types.dart';

typedef SourceDiscoverBridge = DiscoverResult Function(
  int scope,
  String? expression,
  String root,
  int specifiers,
  int limit,
  int timeout,
);

String Function() discoverCurrentRootProvider =
    () => _normalizeAbsolutePath(Directory.current.path);
Map<String, String> Function() discoverEnvironmentProvider =
    () => Platform.environment;
String? Function() discoverPublishedHolonsRootProvider =
    _defaultPublishedHolonsRoot;
String? Function() discoverSiblingsRootProvider = _defaultSiblingsRoot;
String Function() discoverResolvedExecutableProvider =
    () => Platform.resolvedExecutable;
String Function() discoverDartRunWorkingDirectoryProvider =
    () => Directory.current.path;
SourceDiscoverBridge discoverSourceBridge = _defaultSourceDiscoverBridge;

void resetDiscoveryTestOverrides() {
  discoverCurrentRootProvider =
      () => _normalizeAbsolutePath(Directory.current.path);
  discoverEnvironmentProvider = () => Platform.environment;
  discoverPublishedHolonsRootProvider = _defaultPublishedHolonsRoot;
  discoverSiblingsRootProvider = _defaultSiblingsRoot;
  discoverResolvedExecutableProvider = () => Platform.resolvedExecutable;
  discoverDartRunWorkingDirectoryProvider = () => Directory.current.path;
  discoverSourceBridge = _defaultSourceDiscoverBridge;
}

DiscoverResult Discover(
  int scope,
  String? expression,
  String? root,
  int specifiers,
  int limit,
  int timeout,
) {
  if (scope != LOCAL) {
    return DiscoverResult(error: 'scope $scope not supported');
  }
  if (specifiers < 0 || (specifiers & ~ALL) != 0) {
    return DiscoverResult(
      error:
          'invalid specifiers ${_formatSpecifiers(specifiers)}: valid range is 0x00-0x3F',
    );
  }

  var effectiveSpecifiers = specifiers == 0 ? ALL : specifiers;
  if (limit < 0) {
    return const DiscoverResult(found: <HolonRef>[]);
  }

  final normalizedExpression = expression?.trim();
  String? resolvedRoot;

  String resolveRoot() {
    if (resolvedRoot != null) {
      return resolvedRoot!;
    }
    final result = _resolveDiscoverRoot(root);
    if (result.error != null) {
      throw _RootResolutionError(result.error!);
    }
    resolvedRoot = result.root!;
    return resolvedRoot!;
  }

  try {
    if (normalizedExpression != null) {
      final pathResult = _discoverPathExpression(
        normalizedExpression,
        resolveRoot,
        timeout,
      );
      if (pathResult.handled) {
        if (pathResult.error != null) {
          return DiscoverResult(error: pathResult.error);
        }
        return DiscoverResult(
          found: _applyRefLimit(pathResult.found, limit),
        );
      }
    }

    final publishedRoot = _publishedHolonsRoot(root);
    if (publishedRoot != null) {
      return _discoverPublishedPackages(
        publishedRoot,
        normalizedExpression,
        limit,
        timeout,
      );
    }

    final rootResult = _resolveDiscoverRoot(root);
    if (rootResult.error != null) {
      return DiscoverResult(error: rootResult.error);
    }
    resolvedRoot = rootResult.root!;
  } on _RootResolutionError catch (error) {
    return DiscoverResult(error: error.message);
  }

  final found = <HolonRef>[];
  final seen = <String>{};
  final layers = <_Layer>[
    _Layer(
      flag: SIBLINGS,
      name: 'siblings',
      scan: (_, remainingLimit) {
        final siblingsRoot = discoverSiblingsRootProvider();
        if (siblingsRoot == null || siblingsRoot.trim().isEmpty) {
          return const DiscoverResult(found: <HolonRef>[]);
        }
        return _discoverPackageLayer(
          _normalizeAbsolutePath(siblingsRoot),
          'siblings',
          recursive: false,
          timeout: timeout,
        );
      },
    ),
    _Layer(
      flag: CWD,
      name: 'cwd',
      scan: (searchRoot, remainingLimit) => _discoverPackageLayer(
        searchRoot,
        'cwd',
        recursive: true,
        timeout: timeout,
      ),
    ),
    _Layer(
      flag: SOURCE,
      name: 'source',
      scan: (searchRoot, remainingLimit) => discoverSourceBridge(
        LOCAL,
        normalizedExpression,
        searchRoot,
        SOURCE,
        remainingLimit,
        timeout,
      ),
    ),
    _Layer(
      flag: BUILT,
      name: 'built',
      scan: (searchRoot, remainingLimit) => _discoverPackageLayer(
        _joinPath(searchRoot, '.op/build'),
        'built',
        recursive: false,
        timeout: timeout,
      ),
    ),
    _Layer(
      flag: INSTALLED,
      name: 'installed',
      scan: (_, remainingLimit) => _discoverPackageLayer(
        _opbin(),
        'installed',
        recursive: false,
        timeout: timeout,
      ),
    ),
    _Layer(
      flag: CACHED,
      name: 'cached',
      scan: (_, remainingLimit) => _discoverPackageLayer(
        _cacheDir(),
        'cached',
        recursive: true,
        timeout: timeout,
      ),
    ),
  ];

  for (final layer in layers) {
    if ((effectiveSpecifiers & layer.flag) == 0) {
      continue;
    }
    final remainingLimit =
        limit > 0 ? (limit - found.length).clamp(0, limit) : NO_LIMIT;
    if (limit > 0 && remainingLimit == 0) {
      break;
    }

    final result = layer.scan(resolvedRoot!, remainingLimit);
    if (result.error != null) {
      return DiscoverResult(
        error: 'scan ${layer.name} layer: ${result.error}',
      );
    }

    for (final ref in result.found) {
      final key = _refKey(ref);
      if (!seen.add(key)) {
        continue;
      }
      if (!_matchesExpression(ref, normalizedExpression)) {
        continue;
      }
      found.add(ref);
      if (limit > 0 && found.length >= limit) {
        break;
      }
    }
  }

  return DiscoverResult(found: found);
}

String? _publishedHolonsRoot(String? explicitRoot) {
  if (explicitRoot != null) {
    return null;
  }
  final publishedRoot = discoverPublishedHolonsRootProvider()?.trim() ?? '';
  if (publishedRoot.isEmpty) {
    return null;
  }
  final normalized = _normalizeAbsolutePath(publishedRoot);
  if (!Directory(normalized).existsSync()) {
    return null;
  }
  return normalized;
}

DiscoverResult _discoverPublishedPackages(
  String publishedRoot,
  String? expression,
  int limit,
  int timeout,
) {
  final result = _discoverPackageLayer(
    publishedRoot,
    'published',
    recursive: false,
    timeout: timeout,
  );
  if (result.error != null) {
    return result;
  }
  final found = <HolonRef>[];
  final seen = <String>{};
  for (final ref in result.found) {
    final key = _refKey(ref);
    if (!seen.add(key)) {
      continue;
    }
    if (!_matchesExpression(ref, expression)) {
      continue;
    }
    found.add(ref);
    if (limit > 0 && found.length >= limit) {
      break;
    }
  }
  return DiscoverResult(found: found);
}

ResolveResult resolve(
  int scope,
  String expression,
  String? root,
  int specifiers,
  int timeout,
) {
  final result = Discover(scope, expression, root, specifiers, 1, timeout);
  if (result.error != null) {
    return ResolveResult(error: result.error);
  }
  if (result.found.isEmpty) {
    return ResolveResult(error: 'holon "$expression" not found');
  }
  final ref = result.found.first;
  if (ref.error != null && ref.error!.isNotEmpty) {
    return ResolveResult(ref: ref, error: ref.error);
  }
  return ResolveResult(ref: ref);
}

DiscoverResult _discoverPackageLayer(
  String root,
  String origin, {
  required bool recursive,
  required int timeout,
}) {
  final normalizedRoot = _normalizeAbsolutePath(root);
  final directory = Directory(normalizedRoot);
  if (!directory.existsSync()) {
    return const DiscoverResult(found: <HolonRef>[]);
  }

  final dirs = recursive
      ? _packageDirsRecursive(normalizedRoot)
      : _packageDirsDirect(normalizedRoot);
  final recordsByKey = <String, _LayerRecord>{};
  final orderedKeys = <String>[];

  for (final dir in dirs) {
    final record =
        _loadPackageRecord(normalizedRoot, dir, origin, timeout: timeout);
    final key = record.key;
    final existing = recordsByKey[key];
    if (existing != null) {
      if (_shouldReplaceRecord(existing, record)) {
        recordsByKey[key] = record;
      }
      continue;
    }

    recordsByKey[key] = record;
    orderedKeys.add(key);
  }

  final records = <_LayerRecord>[
    for (final key in orderedKeys)
      if (recordsByKey.containsKey(key)) recordsByKey[key]!,
  ]..sort((left, right) {
      final byPath = left.relativePath.compareTo(right.relativePath);
      if (byPath != 0) {
        return byPath;
      }
      return left.uuid.compareTo(right.uuid);
    });

  return DiscoverResult(found: records.map((record) => record.ref).toList());
}

_LayerRecord _loadPackageRecord(
  String root,
  String dir,
  String origin, {
  required int timeout,
}) {
  final normalizedDir = _normalizeAbsolutePath(dir);
  final info = _loadPackageInfoFromJson(normalizedDir);
  if (info != null) {
    return _LayerRecord(
      ref: HolonRef(url: _fileUrl(normalizedDir), info: info),
      dir: normalizedDir,
      relativePath: _relativePath(root, normalizedDir),
      uuid: info.uuid,
      key: info.uuid.trim().isEmpty ? normalizedDir : info.uuid,
    );
  }

  try {
    final probed = _probeHelper('describe-package', normalizedDir, timeout);
    return _LayerRecord(
      ref: HolonRef(url: _fileUrl(normalizedDir), info: probed),
      dir: normalizedDir,
      relativePath: _relativePath(root, normalizedDir),
      uuid: probed.uuid,
      key: probed.uuid.trim().isEmpty ? normalizedDir : probed.uuid,
    );
  } on Object catch (error) {
    return _LayerRecord(
      ref: HolonRef(url: _fileUrl(normalizedDir), error: '$error'),
      dir: normalizedDir,
      relativePath: _relativePath(root, normalizedDir),
      uuid: '',
      key: normalizedDir,
    );
  }
}

HolonInfo? _loadPackageInfoFromJson(String dir) {
  final manifestFile = File(_joinPath(dir, '.holon.json'));
  if (!manifestFile.existsSync()) {
    return null;
  }

  final decoded = jsonDecode(manifestFile.readAsStringSync());
  if (decoded is! Map) {
    return null;
  }
  final json = decoded.map(
    (key, dynamic value) => MapEntry(key.toString(), value),
  );
  final schema = (json['schema'] as String?)?.trim() ?? '';
  if (schema.isNotEmpty && schema != 'holon-package/v1') {
    return null;
  }

  final info = HolonInfo.fromJson(json);
  if (info.slug.isNotEmpty) {
    return info;
  }

  final slug = _slugFromIdentity(info.identity);
  return HolonInfo(
    slug: slug,
    uuid: info.uuid,
    identity: info.identity,
    lang: info.lang,
    runner: info.runner,
    status: info.status,
    kind: info.kind,
    transport: info.transport,
    entrypoint: info.entrypoint,
    architectures: info.architectures,
    hasDist: info.hasDist,
    hasSource: info.hasSource,
  );
}

_PathDiscoverResult _discoverPathExpression(
  String expression,
  String Function() resolveRoot,
  int timeout,
) {
  final candidate = _pathExpressionCandidate(expression, resolveRoot);
  if (!candidate.handled) {
    return const _PathDiscoverResult(handled: false);
  }
  if (candidate.error != null) {
    return _PathDiscoverResult(handled: true, error: candidate.error);
  }

  final path = candidate.path!;
  final type = FileSystemEntity.typeSync(path, followLinks: false);
  if (type == FileSystemEntityType.notFound) {
    return const _PathDiscoverResult(handled: true, found: <HolonRef>[]);
  }

  if (type == FileSystemEntityType.directory) {
    if (_basename(path).endsWith('.holon') ||
        File(_joinPath(path, '.holon.json')).existsSync()) {
      return _PathDiscoverResult(
        handled: true,
        found: <HolonRef>[
          _loadPackageRecord(_dirname(path), path, 'path', timeout: timeout)
              .ref,
        ],
      );
    }

    final source = discoverSourceBridge(
      LOCAL,
      path,
      resolveRoot(),
      SOURCE,
      1,
      timeout,
    );
    if (source.error != null) {
      return _PathDiscoverResult(handled: true, error: source.error);
    }
    return _PathDiscoverResult(handled: true, found: source.found);
  }

  if (_basename(path) == 'holon.proto') {
    final source = discoverSourceBridge(
      LOCAL,
      path,
      resolveRoot(),
      SOURCE,
      1,
      timeout,
    );
    if (source.error != null) {
      return _PathDiscoverResult(handled: true, error: source.error);
    }
    return _PathDiscoverResult(handled: true, found: source.found);
  }

  try {
    final info = _probeHelper('describe-binary', path, timeout);
    return _PathDiscoverResult(
      handled: true,
      found: <HolonRef>[
        HolonRef(url: _fileUrl(path), info: info),
      ],
    );
  } on Object catch (error) {
    return _PathDiscoverResult(
      handled: true,
      found: <HolonRef>[
        HolonRef(url: _fileUrl(path), error: '$error'),
      ],
    );
  }
}

_PathCandidate _pathExpressionCandidate(
  String expression,
  String Function() resolveRoot,
) {
  final trimmed = expression.trim();
  if (trimmed.isEmpty) {
    return const _PathCandidate(handled: false);
  }

  if (trimmed.startsWith('file://')) {
    try {
      return _PathCandidate(
        handled: true,
        path: _normalizeAbsolutePath(Uri.parse(trimmed).toFilePath()),
      );
    } on Object catch (error) {
      return _PathCandidate(handled: true, error: '$error');
    }
  }

  final isPathLike = _isAbsolutePath(trimmed) ||
      trimmed.startsWith('.') ||
      trimmed.contains('/') ||
      trimmed.contains(r'\') ||
      trimmed.endsWith('.holon');
  if (!isPathLike) {
    return const _PathCandidate(handled: false);
  }

  if (_isAbsolutePath(trimmed)) {
    return _PathCandidate(
      handled: true,
      path: _normalizeAbsolutePath(trimmed),
    );
  }

  return _PathCandidate(
    handled: true,
    path: _normalizeAbsolutePath(_joinPath(resolveRoot(), trimmed)),
  );
}

HolonInfo _probeHelper(String command, String path, int timeout) {
  final workingDirectory = discoverDartRunWorkingDirectoryProvider();
  final executable = discoverResolvedExecutableProvider().trim();
  if (!_canRunDartCli(executable)) {
    throw StateError(
      'cannot run Dart discovery probe with resolved executable "$executable"; '
      'package discovery without .holon.json requires the Dart CLI',
    );
  }
  final args = <String>[
    'run',
    _discoveryProbeEntrypoint(workingDirectory),
    command,
    path,
    '$timeout',
  ];

  final result = Process.runSync(
    executable,
    args,
    workingDirectory: workingDirectory,
  );
  if (result.exitCode != 0) {
    final stderr = result.stderr.toString().trim();
    final stdout = result.stdout.toString().trim();
    final message = stderr.isNotEmpty
        ? stderr
        : (stdout.isNotEmpty ? stdout : 'probe failed');
    throw StateError(message);
  }

  final decoded = jsonDecode(result.stdout.toString());
  if (decoded is! Map) {
    throw StateError('probe returned invalid JSON');
  }

  return HolonInfo.fromJson(
    decoded.map((key, dynamic value) => MapEntry(key.toString(), value)),
  );
}

String _discoveryProbeEntrypoint(String workingDirectory) {
  final normalizedWorkingDirectory =
      _normalizeAbsolutePath(workingDirectory.trim());
  final libDir = _joinPath(normalizedWorkingDirectory, 'lib');
  final srcDir = _joinPath(libDir, 'src');
  final helperPath = _joinPath(srcDir, 'discovery_probe.dart');
  if (File(helperPath).existsSync()) {
    return helperPath;
  }
  return 'package:holons/src/discovery_probe.dart';
}

bool _canRunDartCli(String executable) {
  final name = _basename(executable).toLowerCase();
  return name == 'dart' || name == 'dart.exe';
}

DiscoverResult _defaultSourceDiscoverBridge(
  int scope,
  String? expression,
  String root,
  int specifiers,
  int limit,
  int timeout,
) {
  try {
    final args = <String>[
      '--format',
      'json',
      'list',
      '--source',
      '--root',
      root,
    ];
    if (limit > 0) {
      args.addAll(<String>['--limit', '$limit']);
    }
    if (timeout > 0) {
      args.addAll(<String>['--timeout', '$timeout']);
    }

    final result = Process.runSync(
      'op',
      args,
      workingDirectory: discoverDartRunWorkingDirectoryProvider(),
    );
    if (result.exitCode != 0) {
      final stderr = result.stderr.toString().trim();
      final stdout = result.stdout.toString().trim();
      final message = stderr.isNotEmpty
          ? stderr
          : (stdout.isNotEmpty ? stdout : 'op list failed');
      return DiscoverResult(error: 'source discovery offload failed: $message');
    }

    final decoded = jsonDecode(result.stdout.toString());
    if (decoded is! Map) {
      return const DiscoverResult(
        error: 'source discovery offload returned invalid JSON',
      );
    }

    final entries = decoded['entries'];
    if (entries is! List) {
      return const DiscoverResult(found: <HolonRef>[]);
    }

    final refs = <HolonRef>[];
    for (final value in entries.whereType<Map>()) {
      final entry = value.map(
        (key, dynamic item) => MapEntry(key.toString(), item),
      );
      final identityJson = entry['identity'];
      final identity = identityJson is Map<String, dynamic>
          ? IdentityInfo.fromJson(identityJson)
          : const IdentityInfo();
      final relativePath =
          (entry['relativePath'] ?? entry['relative_path'] ?? '.')
              .toString()
              .trim();
      final absolutePath = relativePath == '.' || relativePath.isEmpty
          ? _normalizeAbsolutePath(root)
          : _normalizeAbsolutePath(_joinPath(root, relativePath));
      final info = HolonInfo(
        slug: _slugFromIdentity(identity),
        uuid: _readNestedString(entry, 'identity', 'uuid'),
        identity: identity,
        lang: _readNestedString(entry, 'identity', 'lang'),
        status: _readNestedString(entry, 'identity', 'status'),
        hasSource: true,
      );
      refs.add(HolonRef(url: _fileUrl(absolutePath), info: info));
    }
    return DiscoverResult(found: refs);
  } on ProcessException catch (error) {
    return DiscoverResult(error: 'source discovery offload failed: $error');
  } on FormatException catch (error) {
    return DiscoverResult(error: 'source discovery offload failed: $error');
  }
}

bool _matchesExpression(HolonRef ref, String? expression) {
  if (expression == null) {
    return true;
  }

  final needle = expression.trim();
  if (needle.isEmpty) {
    return false;
  }

  final info = ref.info;
  if (info != null) {
    if (info.slug == needle) {
      return true;
    }
    if (info.uuid.startsWith(needle)) {
      return true;
    }
    if (info.identity.aliases.contains(needle)) {
      return true;
    }
  }

  final path = _pathFromFileUrl(ref.url);
  if (path == null) {
    return false;
  }
  final base = _basename(path).replaceFirst(RegExp(r'\.holon$'), '');
  return base == needle;
}

String _refKey(HolonRef ref) {
  final uuid = ref.info?.uuid.trim() ?? '';
  return uuid.isNotEmpty ? uuid : ref.url;
}

bool _shouldReplaceRecord(_LayerRecord current, _LayerRecord next) {
  return _pathDepth(next.relativePath) < _pathDepth(current.relativePath);
}

List<String> _packageDirsDirect(String root) {
  final directory = Directory(root);
  if (!directory.existsSync()) {
    return const <String>[];
  }

  final dirs = directory
      .listSync(followLinks: false)
      .whereType<Directory>()
      .map((dir) => _normalizeAbsolutePath(dir.path))
      .where((dir) => _basename(dir).endsWith('.holon'))
      .toList()
    ..sort();
  return dirs;
}

List<String> _packageDirsRecursive(String root) {
  final directories = <String>[];
  final stack = <Directory>[Directory(root)];

  while (stack.isNotEmpty) {
    final current = stack.removeLast();

    List<FileSystemEntity> children;
    try {
      children = current.listSync(followLinks: false);
    } on FileSystemException {
      continue;
    }

    for (final entity in children) {
      if (entity is! Directory) {
        continue;
      }
      final path = _normalizeAbsolutePath(entity.path);
      final name = _basename(path);
      if (_shouldSkipDir(root, path, name)) {
        continue;
      }
      if (name.endsWith('.holon')) {
        directories.add(path);
        continue;
      }
      stack.add(entity);
    }
  }

  directories.sort();
  return directories;
}

bool _shouldSkipDir(String root, String path, String name) {
  if (_normalizeAbsolutePath(path) == _normalizeAbsolutePath(root)) {
    return false;
  }
  if (name.endsWith('.holon')) {
    return false;
  }
  if (name == '.git' ||
      name == '.op' ||
      name == 'node_modules' ||
      name == 'vendor' ||
      name == 'build' ||
      name == 'testdata') {
    return true;
  }
  return name.startsWith('.');
}

_RootResolution _resolveDiscoverRoot(String? root) {
  if (root == null) {
    return _RootResolution(
        root: _normalizeAbsolutePath(discoverCurrentRootProvider()));
  }

  final trimmed = root.trim();
  if (trimmed.isEmpty) {
    return const _RootResolution(error: 'root cannot be empty');
  }

  final normalized = _normalizeAbsolutePath(trimmed);
  final directory = Directory(normalized);
  if (!directory.existsSync()) {
    return _RootResolution(error: 'root "$trimmed" is not a directory');
  }
  return _RootResolution(root: normalized);
}

String _formatSpecifiers(int specifiers) {
  final masked = specifiers & 0xFF;
  return '0x${masked.toRadixString(16).padLeft(2, '0').toUpperCase()}';
}

List<HolonRef> _applyRefLimit(List<HolonRef> refs, int limit) {
  if (limit <= 0 || refs.length <= limit) {
    return refs;
  }
  return refs.sublist(0, limit);
}

String _slugFromIdentity(IdentityInfo identity) {
  final family = identity.familyName.trim().replaceFirst(RegExp(r'\?$'), '');
  return '${identity.givenName.trim()}-$family'
      .trim()
      .toLowerCase()
      .replaceAll(' ', '-')
      .replaceAll(RegExp(r'^-+|-+$'), '');
}

String _readNestedString(
  Map<String, dynamic> json,
  String key,
  String nestedKey,
) {
  final nested = json[key];
  if (nested is! Map) {
    return '';
  }
  final value = nested[nestedKey];
  return value is String ? value.trim() : '';
}

String? _defaultSiblingsRoot() {
  return _defaultPublishedHolonsRoot();
}

String? _defaultPublishedHolonsRoot() {
  final executable = discoverResolvedExecutableProvider().trim();
  if (executable.isEmpty) {
    return null;
  }
  var current = Directory(_dirname(executable));
  while (true) {
    final path = _normalizeAbsolutePath(current.path);
    for (final candidate in _publishedSiblingsCandidates(path)) {
      if (Directory(candidate).existsSync()) {
        return candidate;
      }
    }
    final parent = current.parent;
    if (_normalizeAbsolutePath(parent.path) == path) {
      break;
    }
    current = parent;
  }
  return null;
}

Iterable<String> _publishedSiblingsCandidates(String path) sync* {
  if (_basename(path).toLowerCase().endsWith('.app')) {
    yield _joinPath(path, 'Contents/Resources/Holons');
  }
  yield _joinPath(path, 'data/Holons');
}

String _oppath() {
  final env = discoverEnvironmentProvider();
  final configured = (env['OPPATH'] ?? '').trim();
  if (configured.isNotEmpty) {
    return _normalizeAbsolutePath(configured);
  }

  final home = (env['HOME'] ?? '').trim();
  if (home.isEmpty) {
    return _normalizeAbsolutePath('.op');
  }
  return _normalizeAbsolutePath(_joinPath(home, '.op'));
}

String _opbin() {
  final env = discoverEnvironmentProvider();
  final configured = (env['OPBIN'] ?? '').trim();
  if (configured.isNotEmpty) {
    return _normalizeAbsolutePath(configured);
  }
  return _joinPath(_oppath(), 'bin');
}

String _cacheDir() => _joinPath(_oppath(), 'cache');

String _fileUrl(String path) => Uri.file(path).toString();

String? _pathFromFileUrl(String raw) {
  if (!raw.startsWith('file://')) {
    return null;
  }
  return _normalizeAbsolutePath(Uri.parse(raw).toFilePath());
}

bool _isAbsolutePath(String path) {
  if (path.startsWith('/')) {
    return true;
  }
  return RegExp(r'^[A-Za-z]:[\\/]').hasMatch(path);
}

String _joinPath(String left, String right) {
  final separator = Platform.pathSeparator;
  final normalizedLeft =
      left.endsWith(separator) ? left.substring(0, left.length - 1) : left;
  final normalizedRight =
      right.startsWith(separator) ? right.substring(1) : right;
  return '$normalizedLeft$separator$normalizedRight';
}

String _relativePath(String root, String dir) {
  final normalizedRoot = _normalizeAbsolutePath(root);
  final normalizedDir = _normalizeAbsolutePath(dir);
  if (normalizedDir == normalizedRoot) {
    return '.';
  }
  final prefix =
      normalizedRoot.endsWith('/') ? normalizedRoot : '$normalizedRoot/';
  if (normalizedDir.startsWith(prefix)) {
    return normalizedDir.substring(prefix.length);
  }
  return normalizedDir;
}

int _pathDepth(String relativePath) {
  final trimmed = relativePath
      .trim()
      .replaceAll('\\', '/')
      .replaceAll(RegExp(r'^/+|/+$'), '');
  if (trimmed.isEmpty || trimmed == '.') {
    return 0;
  }
  return trimmed.split('/').length;
}

String _normalizeAbsolutePath(String path) {
  final file = File(path);
  final directory = Directory(path);
  final absolutePath =
      (directory.path.endsWith('/') || Directory(path).existsSync())
          ? directory.absolute.path
          : file.absolute.path;
  final normalized = absolutePath.replaceAll('\\', '/');
  if (normalized.length > 1 && normalized.endsWith('/')) {
    return normalized.substring(0, normalized.length - 1);
  }
  return normalized;
}

String _basename(String path) {
  final normalized = path.replaceAll('\\', '/');
  final trimmed = normalized.endsWith('/') && normalized.length > 1
      ? normalized.substring(0, normalized.length - 1)
      : normalized;
  final index = trimmed.lastIndexOf('/');
  return index >= 0 ? trimmed.substring(index + 1) : trimmed;
}

String _dirname(String path) {
  final normalized = path.replaceAll('\\', '/');
  final trimmed = normalized.endsWith('/') && normalized.length > 1
      ? normalized.substring(0, normalized.length - 1)
      : normalized;
  final index = trimmed.lastIndexOf('/');
  if (index <= 0) {
    return index == 0 ? '/' : '.';
  }
  return trimmed.substring(0, index);
}

final class _Layer {
  final int flag;
  final String name;
  final DiscoverResult Function(String root, int remainingLimit) scan;

  const _Layer({
    required this.flag,
    required this.name,
    required this.scan,
  });
}

final class _LayerRecord {
  final HolonRef ref;
  final String dir;
  final String relativePath;
  final String uuid;
  final String key;

  const _LayerRecord({
    required this.ref,
    required this.dir,
    required this.relativePath,
    required this.uuid,
    required this.key,
  });
}

final class _PathDiscoverResult {
  final bool handled;
  final List<HolonRef> found;
  final String? error;

  const _PathDiscoverResult({
    required this.handled,
    this.found = const <HolonRef>[],
    this.error,
  });
}

final class _PathCandidate {
  final bool handled;
  final String? path;
  final String? error;

  const _PathCandidate({
    required this.handled,
    this.path,
    this.error,
  });
}

final class _RootResolution {
  final String? root;
  final String? error;

  const _RootResolution({
    this.root,
    this.error,
  });
}

final class _RootResolutionError implements Exception {
  final String message;

  const _RootResolutionError(this.message);
}
