import 'dart:io';

/// Resolve a declared member's binary relative to the calling composite's own
/// executable.
String member(String id) {
  return memberFromExecutable(Platform.resolvedExecutable, id);
}

String memberFromExecutable(String selfPath, String id) {
  final trimmed = id.trim();
  if (trimmed.isEmpty) {
    throw ArgumentError.value(id, 'id', 'member id is required');
  }
  final self = File(selfPath).absolute;
  final memberDir = Directory(
    '${self.parent.path}${Platform.pathSeparator}holons${Platform.pathSeparator}$trimmed',
  );
  for (final entity in memberDir.listSync(followLinks: false)) {
    if (entity is File && _isExecutable(entity)) {
      return entity.absolute.path;
    }
  }
  throw FileSystemException('no executable found', memberDir.path);
}

bool _isExecutable(File file) {
  final stat = file.statSync();
  if (stat.type != FileSystemEntityType.file) {
    return false;
  }
  if (Platform.isWindows) {
    return file.uri.pathSegments.last.toLowerCase().endsWith('.exe');
  }
  return stat.mode & 0x49 != 0; // 0o111
}
