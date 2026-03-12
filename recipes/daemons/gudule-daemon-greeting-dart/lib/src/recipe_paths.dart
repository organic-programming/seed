import 'dart:io';

String findRecipeRoot() {
  final starts = <String>{
    Directory.current.absolute.path,
    File(Platform.resolvedExecutable).absolute.parent.path,
    File.fromUri(Platform.script).absolute.parent.path,
  };

  for (final start in starts) {
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
  }

  throw StateError('could not locate gudule-daemon-greeting-dart recipe root');
}
