import 'dart:io';

const String protoManifestFileName = 'holon.proto';

/// Parsed holon identity from holon.proto.
class HolonIdentity {
  final String uuid;
  final String givenName;
  final String familyName;
  final String motto;
  final String composer;
  final String clade;
  final String status;
  final String born;
  final String lang;
  final List<String> parents;
  final String reproduction;
  final String generatedBy;
  final String protoStatus;
  final List<String> aliases;

  const HolonIdentity({
    this.uuid = '',
    this.givenName = '',
    this.familyName = '',
    this.motto = '',
    this.composer = '',
    this.clade = '',
    this.status = '',
    this.born = '',
    this.lang = '',
    this.parents = const <String>[],
    this.reproduction = '',
    this.generatedBy = '',
    this.protoStatus = '',
    this.aliases = const <String>[],
  });

  String slug() {
    final given = givenName.trim();
    final family = familyName.trim().replaceFirst(RegExp(r'\?$'), '');
    if (given.isEmpty && family.isEmpty) {
      return '';
    }

    return '$given-$family'
        .trim()
        .toLowerCase()
        .replaceAll(' ', '-')
        .replaceAll(RegExp(r'^-+|-+$'), '');
  }
}

class ResolvedManifest {
  final HolonIdentity identity;
  final String sourcePath;
  final String kind;
  final String buildRunner;
  final String buildMain;
  final String artifactBinary;
  final String artifactPrimary;

  const ResolvedManifest({
    this.identity = const HolonIdentity(),
    this.sourcePath = '',
    this.kind = '',
    this.buildRunner = '',
    this.buildMain = '',
    this.artifactBinary = '',
    this.artifactPrimary = '',
  });
}

/// Parse a holon.proto manifest file.
HolonIdentity parseHolon(String path) {
  return resolveProtoFile(path).identity;
}

ResolvedManifest parseManifest(String path) {
  return resolveProtoFile(path);
}

ResolvedManifest resolve(String root) {
  return resolveProtoFile(resolveManifestPath(root));
}

ResolvedManifest resolveProtoFile(String path) {
  final text = File(path).readAsStringSync();
  final manifestBlock = _extractManifestBlock(text);
  if (manifestBlock == null) {
    throw FormatException(
        '$path: missing holons.v1.manifest option in holon.proto');
  }

  final identityBlock = _extractBlock('identity', manifestBlock) ?? '';
  final lineageBlock = _extractBlock('lineage', manifestBlock) ?? '';
  final buildBlock = _extractBlock('build', manifestBlock) ?? '';
  final artifactsBlock = _extractBlock('artifacts', manifestBlock) ?? '';

  return ResolvedManifest(
    identity: HolonIdentity(
      uuid: _scalar('uuid', identityBlock),
      givenName: _scalar('given_name', identityBlock),
      familyName: _scalar('family_name', identityBlock),
      motto: _scalar('motto', identityBlock),
      composer: _scalar('composer', identityBlock),
      clade: _scalar('clade', identityBlock),
      status: _scalar('status', identityBlock),
      born: _scalar('born', identityBlock),
      lang: _scalar('lang', manifestBlock),
      parents: _stringList('parents', lineageBlock),
      reproduction: _scalar('reproduction', lineageBlock),
      generatedBy: _scalar('generated_by', lineageBlock),
      protoStatus: _scalar('proto_status', identityBlock),
      aliases: _stringList('aliases', identityBlock),
    ),
    sourcePath: _normalizeAbsolutePath(path),
    kind: _scalar('kind', manifestBlock),
    buildRunner: _scalar('runner', buildBlock),
    buildMain: _scalar('main', buildBlock),
    artifactBinary: _scalar('binary', artifactsBlock),
    artifactPrimary: _scalar('primary', artifactsBlock),
  );
}

String? findHolonProto(String root) {
  final normalized = _normalizeAbsolutePath(root);
  final type = FileSystemEntity.typeSync(normalized, followLinks: false);
  if (type == FileSystemEntityType.file) {
    return _basename(normalized) == protoManifestFileName ? normalized : null;
  }
  if (type != FileSystemEntityType.directory) {
    return null;
  }

  final direct = '$normalized/$protoManifestFileName';
  if (File(direct).existsSync()) {
    return _normalizeAbsolutePath(direct);
  }

  final apiV1 = '$normalized/api/v1/$protoManifestFileName';
  if (File(apiV1).existsSync()) {
    return _normalizeAbsolutePath(apiV1);
  }

  final candidates = <String>[];
  for (final entity
      in Directory(normalized).listSync(recursive: true, followLinks: false)) {
    if (entity is File && _basename(entity.path) == protoManifestFileName) {
      candidates.add(_normalizeAbsolutePath(entity.path));
    }
  }
  candidates.sort();
  return candidates.isEmpty ? null : candidates.first;
}

String resolveManifestPath(String root) {
  final normalized = _normalizeAbsolutePath(root);
  final searchRoots = <String>[normalized];
  final parent = _dirname(normalized);
  if (_basename(normalized) == 'protos') {
    searchRoots.add(parent);
  } else if (!searchRoots.contains(parent)) {
    searchRoots.add(parent);
  }

  for (final candidateRoot in searchRoots) {
    final candidate = findHolonProto(candidateRoot);
    if (candidate != null) {
      return candidate;
    }
  }

  throw FormatException('no holon.proto found near $normalized');
}

String? _extractManifestBlock(String source) {
  final match = RegExp(
    r'option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{',
    multiLine: true,
  ).firstMatch(source);
  if (match == null) {
    return null;
  }
  final braceIndex = source.indexOf('{', match.start);
  if (braceIndex < 0) {
    return null;
  }
  return _balancedBlockContents(source, braceIndex);
}

String? _extractBlock(String name, String source) {
  final match = RegExp(
    '\\b${RegExp.escape(name)}\\s*:\\s*\\{',
    multiLine: true,
  ).firstMatch(source);
  if (match == null) {
    return null;
  }
  final braceIndex = source.indexOf('{', match.start);
  if (braceIndex < 0) {
    return null;
  }
  return _balancedBlockContents(source, braceIndex);
}

String _scalar(String name, String source) {
  final quoted = RegExp(
    '\\b${RegExp.escape(name)}\\s*:\\s*"((?:[^"\\\\]|\\\\.)*)"',
    multiLine: true,
  ).firstMatch(source);
  if (quoted != null) {
    return _unescapeProtoString(quoted.group(1)!);
  }

  final bare = RegExp(
    '\\b${RegExp.escape(name)}\\s*:\\s*([^\\s,\\]\\}]+)',
    multiLine: true,
  ).firstMatch(source);
  return bare?.group(1) ?? '';
}

List<String> _stringList(String name, String source) {
  final match = RegExp(
    '\\b${RegExp.escape(name)}\\s*:\\s*\\[(.*?)\\]',
    multiLine: true,
    dotAll: true,
  ).firstMatch(source);
  if (match == null) {
    return const <String>[];
  }

  final values = <String>[];
  final tokenPattern = RegExp(r'"((?:[^"\\]|\\.)*)"|([^\s,\]]+)');
  for (final token in tokenPattern.allMatches(match.group(1)!)) {
    final quoted = token.group(1);
    final bare = token.group(2);
    if (quoted != null) {
      values.add(_unescapeProtoString(quoted));
    } else if (bare != null) {
      values.add(bare);
    }
  }
  return values;
}

String? _balancedBlockContents(String source, int openingBrace) {
  var depth = 0;
  var insideString = false;
  var escaped = false;
  final contentStart = openingBrace + 1;

  for (var index = openingBrace; index < source.length; index++) {
    final char = source[index];
    if (insideString) {
      if (escaped) {
        escaped = false;
      } else if (char == '\\') {
        escaped = true;
      } else if (char == '"') {
        insideString = false;
      }
      continue;
    }

    if (char == '"') {
      insideString = true;
    } else if (char == '{') {
      depth += 1;
    } else if (char == '}') {
      depth -= 1;
      if (depth == 0) {
        return source.substring(contentStart, index);
      }
    }
  }

  return null;
}

String _unescapeProtoString(String value) {
  return value.replaceAll(r'\"', '"').replaceAll(r'\\', '\\');
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
