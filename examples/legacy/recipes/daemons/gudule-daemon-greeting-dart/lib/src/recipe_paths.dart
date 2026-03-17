import 'dart:io';

String? locateRecipeRoot() {
  final starts = <String>{
    Directory.current.absolute.path,
    File(Platform.resolvedExecutable).absolute.parent.path,
    File.fromUri(Platform.script).absolute.parent.path,
  };

  for (final start in starts) {
    final root = _searchRecipeRoot(start);
    if (root != null) {
      return root;
    }
  }

  return null;
}

String findRecipeRoot() {
  final root = locateRecipeRoot();
  if (root != null) {
    return root;
  }

  throw StateError(
    'could not locate gudule-daemon-greeting-dart recipe root',
  );
}

String? _searchRecipeRoot(String start) {
  var current = Directory(start).absolute;
  while (true) {
    final holonYaml = File('${current.path}/holon.yaml');
    final pubspec = File('${current.path}/pubspec.yaml');
    if (holonYaml.existsSync() && pubspec.existsSync()) {
      return current.path;
    }

    final parent = current.parent;
    if (parent.path == current.path) {
      break;
    }
    current = parent;
  }
  return null;
}
