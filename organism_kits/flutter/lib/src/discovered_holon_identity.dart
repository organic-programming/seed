import 'dart:io';

class DiscoveredHolonIdentity {
  const DiscoveredHolonIdentity({
    required this.slug,
    required this.familyName,
    required this.binaryName,
    required this.buildRunner,
    required this.holonUuid,
    required this.sourceKind,
    required this.discoveryPath,
    required this.hasSource,
  });

  final String slug;
  final String familyName;
  final String binaryName;
  final String buildRunner;
  final String holonUuid;
  final String sourceKind;
  final String discoveryPath;
  final bool hasSource;

  static DiscoveredHolonIdentity? fromDiscovered(dynamic ref) {
    final info = ref.info;
    if (info == null) {
      return null;
    }
    final slug = info.slug.trim();
    final entrypoint = info.entrypoint.trim();
    final discoveryPath = discoveryPathFromUrl(ref.url as String);
    return DiscoveredHolonIdentity(
      slug: slug,
      familyName: info.identity.familyName.trim(),
      binaryName: entrypoint.isEmpty
          ? slug
          : entrypoint.split(Platform.pathSeparator).last,
      buildRunner: info.runner.trim(),
      holonUuid: info.uuid.trim(),
      sourceKind: sourceKindForDiscoveryPath(discoveryPath),
      discoveryPath: discoveryPath,
      hasSource: info.hasSource,
    );
  }
}

String discoveryPathFromUrl(String url) {
  final uri = Uri.tryParse(url);
  if (uri != null && uri.scheme == 'file') {
    return uri.toFilePath();
  }
  return url;
}

String sourceKindForDiscoveryPath(String path) {
  if (path.contains('.op${Platform.pathSeparator}build')) {
    return 'built';
  }
  if (path.contains(
    '${Platform.pathSeparator}Holons${Platform.pathSeparator}',
  )) {
    return 'siblings';
  }
  return 'source';
}
