import 'dart:io';

class GreetingEndpoint {
  final String? target;
  final String? bundledBinaryPath;

  const GreetingEndpoint({
    this.target,
    this.bundledBinaryPath,
  });
}

class GreetingTargetResolver {
  static const String daemonSlug = 'gudule-greeting-daemon-go';
  static const String daemonBinary = 'gudule-daemon-greeting-go';

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

    final bundledBinaryPath = _resolveBundledBinaryPath();
    if (bundledBinaryPath != null) {
      return GreetingEndpoint(bundledBinaryPath: bundledBinaryPath);
    }

    return const GreetingEndpoint();
  }

  String? _resolveBundledBinaryPath() {
    final daemonFileName =
        Platform.isWindows ? '$daemonBinary.exe' : daemonBinary;
    final executable = File(executablePath);

    if (Platform.isMacOS) {
      final bundled = File(
        '${executable.parent.parent.path}/Resources/$daemonFileName',
      );
      if (bundled.existsSync()) {
        return bundled.path;
      }
    }

    if (!Platform.isMacOS) {
      final sibling = File('${executable.parent.path}/$daemonFileName');
      if (sibling.existsSync()) {
        return sibling.path;
      }
    }

    final localBuild = File('$currentDirectoryPath/build/$daemonFileName');
    if (localBuild.existsSync()) {
      return localBuild.path;
    }

    final parentBuild = File('$currentDirectoryPath/../build/$daemonFileName');
    if (parentBuild.existsSync()) {
      return parentBuild.path;
    }

    final siblingDaemon = File(
      '$currentDirectoryPath/../../daemons/$daemonBinary/.op/build/bin/$daemonFileName',
    );
    if (siblingDaemon.existsSync()) {
      return siblingDaemon.path;
    }

    return null;
  }
}
