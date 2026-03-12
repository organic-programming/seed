import 'dart:io';

class GreetingDaemonIdentity {
  static const String binaryPrefix = 'gudule-daemon-greeting-';

  final String slug;
  final String binaryName;
  final String familyName;

  const GreetingDaemonIdentity({
    required this.slug,
    required this.binaryName,
    required this.familyName,
  });

  factory GreetingDaemonIdentity.fromBinaryPath(String binaryPath) {
    return GreetingDaemonIdentity.fromBinaryName(
      File(binaryPath).uri.pathSegments.last,
    );
  }

  factory GreetingDaemonIdentity.fromBinaryName(String binaryName) {
    final normalized = binaryName.endsWith('.exe')
        ? binaryName.substring(0, binaryName.length - 4)
        : binaryName;
    if (!normalized.startsWith(binaryPrefix)) {
      return GreetingDaemonIdentity(
        slug: normalized,
        binaryName: normalized,
        familyName: 'Greeting-Daemon',
      );
    }

    final variant = normalized.substring(binaryPrefix.length);
    return GreetingDaemonIdentity(
      slug: 'gudule-greeting-daemon-$variant',
      binaryName: normalized,
      familyName: 'Greeting-Daemon-${_displayVariant(variant)}',
    );
  }

  static String _displayVariant(String variant) {
    const overrides = <String, String>{
      'cpp': 'CPP',
      'js': 'JS',
      'qt': 'Qt',
    };

    return variant.split('-').where((token) => token.isNotEmpty).map((token) {
      final override = overrides[token];
      if (override != null) {
        return override;
      }
      return '${token[0].toUpperCase()}${token.substring(1)}';
    }).join('-');
  }
}

class GreetingEndpoint {
  final String? target;
  final String? bundledBinaryPath;
  final GreetingDaemonIdentity? daemon;

  const GreetingEndpoint({
    this.target,
    this.bundledBinaryPath,
    this.daemon,
  });
}

class GreetingTargetResolver {
  final String compileTimeTarget;
  final Map<String, String> environment;
  final String executablePath;
  final String currentDirectoryPath;

  GreetingTargetResolver({
    String? compileTimeTarget,
    Map<String, String>? environment,
    String? executablePath,
    String? currentDirectoryPath,
  })  : compileTimeTarget = (compileTimeTarget ??
                const String.fromEnvironment('GREETING_TARGET'))
            .trim(),
        environment = environment ?? Platform.environment,
        executablePath = executablePath ?? Platform.resolvedExecutable,
        currentDirectoryPath = currentDirectoryPath ?? Directory.current.path;

  GreetingEndpoint resolve() {
    if (compileTimeTarget.isNotEmpty) {
      return GreetingEndpoint(target: compileTimeTarget);
    }

    final runtimeTarget = (environment['GREETING_TARGET'] ?? '').trim();
    if (runtimeTarget.isNotEmpty) {
      return GreetingEndpoint(target: runtimeTarget);
    }

    final bundledBinary = _resolveBundledBinary();
    if (bundledBinary != null) {
      return GreetingEndpoint(
        bundledBinaryPath: bundledBinary.path,
        daemon: bundledBinary.daemon,
      );
    }

    return const GreetingEndpoint();
  }

  _BundledBinary? _resolveBundledBinary() {
    final executable = File(executablePath);

    if (Platform.isMacOS) {
      final bundled = _findBundledBinary(
        Directory('${executable.parent.parent.path}/Resources'),
      );
      if (bundled != null) {
        return bundled;
      }
    }

    if (!Platform.isMacOS) {
      final sibling = _findBundledBinary(executable.parent);
      if (sibling != null) {
        return sibling;
      }
    }

    final localBuild =
        _findBundledBinary(Directory('$currentDirectoryPath/build'));
    if (localBuild != null) {
      return localBuild;
    }

    final parentBuild = _findBundledBinary(
      Directory('$currentDirectoryPath/../build'),
    );
    if (parentBuild != null) {
      return parentBuild;
    }

    return _findSiblingDaemonBinary();
  }

  _BundledBinary? _findBundledBinary(Directory directory) {
    if (!directory.existsSync()) {
      return null;
    }

    final matches = directory
        .listSync()
        .whereType<File>()
        .where((file) => _isGreetingBinary(file.uri.pathSegments.last))
        .toList()
      ..sort((left, right) => left.path.compareTo(right.path));
    if (matches.isEmpty) {
      return null;
    }

    final file = matches.first;
    return _BundledBinary(
      path: file.path,
      daemon: GreetingDaemonIdentity.fromBinaryPath(file.path),
    );
  }

  _BundledBinary? _findSiblingDaemonBinary() {
    final daemonsDir = Directory('$currentDirectoryPath/../../daemons');
    if (!daemonsDir.existsSync()) {
      return null;
    }

    final matches = daemonsDir
        .listSync()
        .whereType<Directory>()
        .where((dir) => _isGreetingBinary(dir.uri.pathSegments.last))
        .map((dir) {
          final binaryName = dir.uri.pathSegments.last;
          final fileName = Platform.isWindows ? '$binaryName.exe' : binaryName;
          return _BundledBinary(
            path: '${dir.path}/.op/build/bin/$fileName',
            daemon: GreetingDaemonIdentity.fromBinaryName(binaryName),
          );
        })
        .where((candidate) => File(candidate.path).existsSync())
        .toList()
      ..sort((left, right) => left.path.compareTo(right.path));

    return matches.isEmpty ? null : matches.first;
  }

  bool _isGreetingBinary(String fileName) {
    final normalized = fileName.endsWith('.exe')
        ? fileName.substring(0, fileName.length - 4)
        : fileName;
    return normalized.startsWith(GreetingDaemonIdentity.binaryPrefix);
  }
}

class _BundledBinary {
  final String path;
  final GreetingDaemonIdentity daemon;

  const _BundledBinary({
    required this.path,
    required this.daemon,
  });
}
