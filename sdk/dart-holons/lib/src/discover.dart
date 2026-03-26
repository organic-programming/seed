import 'dart:io';

import 'identity.dart';

class HolonBuild {
  final String runner;
  final String main;

  const HolonBuild({
    this.runner = '',
    this.main = '',
  });
}

class HolonArtifacts {
  final String binary;
  final String primary;

  const HolonArtifacts({
    this.binary = '',
    this.primary = '',
  });
}

class HolonManifest {
  final String kind;
  final HolonBuild build;
  final HolonArtifacts artifacts;

  const HolonManifest({
    this.kind = '',
    this.build = const HolonBuild(),
    this.artifacts = const HolonArtifacts(),
  });
}

class HolonEntry {
  final String slug;
  final String uuid;
  final String dir;
  final String relativePath;
  final String origin;
  final HolonIdentity identity;
  final HolonManifest? manifest;

  const HolonEntry({
    required this.slug,
    required this.uuid,
    required this.dir,
    required this.relativePath,
    required this.origin,
    required this.identity,
    required this.manifest,
  });
}

Future<List<HolonEntry>> discover(String root) async {
  return _discoverInRoot(root, 'local');
}

Future<List<HolonEntry>> discoverLocal() async {
  return discover(Directory.current.path);
}

Future<List<HolonEntry>> discoverAll() async {
  final roots = <({String path, String origin})>[
    (path: Directory.current.path, origin: 'local'),
    (path: _opbin(), origin: r'$OPBIN'),
    (path: _cacheDir(), origin: 'cache'),
  ];

  final seen = <String>{};
  final entries = <HolonEntry>[];
  for (final root in roots) {
    final discovered = await _discoverInRoot(root.path, root.origin);
    for (final entry in discovered) {
      final key = entry.uuid.trim().isEmpty ? entry.dir : entry.uuid;
      if (seen.add(key)) {
        entries.add(entry);
      }
    }
  }

  return entries;
}

Future<HolonEntry?> findBySlug(String slug) async {
  final needle = slug.trim();
  if (needle.isEmpty) {
    return null;
  }

  HolonEntry? match;
  for (final entry in await discoverAll()) {
    if (entry.slug != needle) {
      continue;
    }
    if (match != null && match.uuid != entry.uuid) {
      throw StateError('ambiguous holon "$needle"');
    }
    match = entry;
  }
  return match;
}

Future<HolonEntry?> findByUUID(String prefix) async {
  final needle = prefix.trim();
  if (needle.isEmpty) {
    return null;
  }

  HolonEntry? match;
  for (final entry in await discoverAll()) {
    if (!entry.uuid.startsWith(needle)) {
      continue;
    }
    if (match != null && match.uuid != entry.uuid) {
      throw StateError('ambiguous UUID prefix "$needle"');
    }
    match = entry;
  }
  return match;
}

Future<List<HolonEntry>> _discoverInRoot(String root, String origin) async {
  final normalizedRoot = _normalizeAbsolutePath(root);
  final directory = Directory(normalizedRoot);
  if (!directory.existsSync()) {
    return <HolonEntry>[];
  }

  final entriesByKey = <String, HolonEntry>{};
  final orderedKeys = <String>[];

  void scan(Directory current) {
    List<FileSystemEntity> entities;
    try {
      entities = current.listSync(followLinks: false);
    } on FileSystemException {
      return;
    }

    for (final entity in entities) {
      final path = _normalizeAbsolutePath(entity.path);
      final name = _basename(path);
      if (entity is Directory) {
        if (_shouldSkipDir(normalizedRoot, path, name)) {
          continue;
        }
        scan(entity);
        continue;
      }
      if (entity is! File || name != 'holon.proto') {
        continue;
      }

      try {
        final resolved = resolveProtoFile(path);
        final dir = _manifestRoot(path);
        final entry = HolonEntry(
          slug: resolved.identity.slug(),
          uuid: resolved.identity.uuid,
          dir: dir,
          relativePath: _relativePath(normalizedRoot, dir),
          origin: origin,
          identity: resolved.identity,
          manifest: HolonManifest(
            kind: resolved.kind,
            build: HolonBuild(
              runner: resolved.buildRunner,
              main: resolved.buildMain,
            ),
            artifacts: HolonArtifacts(
              binary: resolved.artifactBinary,
              primary: resolved.artifactPrimary,
            ),
          ),
        );

        final key = entry.uuid.trim().isEmpty ? entry.dir : entry.uuid;
        final existing = entriesByKey[key];
        if (existing != null) {
          if (_pathDepth(entry.relativePath) <
              _pathDepth(existing.relativePath)) {
            entriesByKey[key] = entry;
          }
          continue;
        }

        entriesByKey[key] = entry;
        orderedKeys.add(key);
      } on Object {
        continue;
      }
    }
  }

  scan(directory);

  final entries = <HolonEntry>[
    for (final key in orderedKeys)
      if (entriesByKey.containsKey(key)) entriesByKey[key]!,
  ];
  entries.sort((left, right) {
    final byPath = left.relativePath.compareTo(right.relativePath);
    if (byPath != 0) {
      return byPath;
    }
    return left.uuid.compareTo(right.uuid);
  });
  return entries;
}

bool _shouldSkipDir(String root, String path, String name) {
  if (path == root) {
    return false;
  }
  if (name == '.git' ||
      name == '.op' ||
      name == 'node_modules' ||
      name == 'vendor' ||
      name == 'build') {
    return true;
  }
  return name.startsWith('.');
}

String _relativePath(String root, String dir) {
  if (dir == root) {
    return '.';
  }
  final prefix = root.endsWith('/') ? root : '$root/';
  if (dir.startsWith(prefix)) {
    return dir.substring(prefix.length);
  }
  return dir;
}

String _manifestRoot(String manifestPath) {
  final manifestDir = _dirname(manifestPath);
  final versionDir = _basename(manifestDir);
  final apiDir = _basename(_dirname(manifestDir));
  if (RegExp(r'^v[0-9]+(?:[A-Za-z0-9._-]*)?$').hasMatch(versionDir) &&
      apiDir == 'api') {
    return _dirname(_dirname(manifestDir));
  }
  return manifestDir;
}

int _pathDepth(String relativePath) {
  final trimmed = relativePath.trim().replaceAll(RegExp(r'^/+|/+$'), '');
  if (trimmed.isEmpty || trimmed == '.') {
    return 0;
  }
  return trimmed.split('/').length;
}

String _oppath() {
  final env = Platform.environment['OPPATH']?.trim();
  if (env != null && env.isNotEmpty) {
    return _normalizeAbsolutePath(env);
  }

  final home = Platform.environment['HOME']?.trim();
  if (home != null && home.isNotEmpty) {
    return _normalizeAbsolutePath('$home/.op');
  }
  return _normalizeAbsolutePath('.op');
}

String _opbin() {
  final env = Platform.environment['OPBIN']?.trim();
  if (env != null && env.isNotEmpty) {
    return _normalizeAbsolutePath(env);
  }
  return _normalizeAbsolutePath('${_oppath()}/bin');
}

String _cacheDir() {
  return _normalizeAbsolutePath('${_oppath()}/cache');
}

String _normalizeAbsolutePath(String path) {
  final normalized = Directory(path).absolute.path.replaceAll('\\', '/');
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
  if (index < 0) {
    return trimmed;
  }
  return trimmed.substring(index + 1);
}

String _dirname(String path) {
  final normalized = path.replaceAll('\\', '/');
  final index = normalized.lastIndexOf('/');
  if (index <= 0) {
    return index == 0 ? '/' : '.';
  }
  return normalized.substring(0, index);
}
