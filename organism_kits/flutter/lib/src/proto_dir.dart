import 'dart:io';

import 'package:path/path.dart' as p;

String? findAppProtoDir() {
  for (final candidate in _sourceProtoDirCandidates()) {
    if (_containsHolonProto(candidate)) {
      return candidate;
    }
  }
  for (final candidate in _packagedProtoDirCandidates()) {
    if (_containsHolonProto(candidate)) {
      return candidate;
    }
  }
  return null;
}

Iterable<String> _sourceProtoDirCandidates() sync* {
  var current = Directory.current.absolute;
  while (true) {
    yield p.normalize(p.join(current.path, 'api'));

    final parent = current.parent.absolute;
    if (p.equals(parent.path, current.path)) {
      break;
    }
    current = parent;
  }
}

Iterable<String> _packagedProtoDirCandidates() sync* {
  final executable = Platform.resolvedExecutable.trim();
  if (executable.isEmpty) {
    return;
  }

  var current = Directory(p.dirname(executable)).absolute;
  while (true) {
    final currentPath = p.normalize(current.path);
    final currentName = p.basename(currentPath).toLowerCase();

    if (currentName.endsWith('.app')) {
      yield p.join(currentPath, 'Contents', 'Resources', 'AppProto');
    }

    yield p.join(currentPath, 'data', 'AppProto');
    yield p.join(currentPath, 'AppProto');

    final parent = current.parent.absolute;
    if (p.equals(parent.path, current.path)) {
      break;
    }
    current = parent;
  }
}

bool _containsHolonProto(String protoDir) {
  return File(p.join(protoDir, 'v1', 'holon.proto')).existsSync();
}
