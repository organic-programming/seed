import 'package:holons_app/holons_app.dart';

class GabrielHolonIdentity {
  const GabrielHolonIdentity({
    required this.slug,
    required this.familyName,
    required this.binaryName,
    required this.buildRunner,
    required this.displayName,
    required this.sortRank,
    required this.holonUuid,
    required this.born,
    required this.sourceKind,
    required this.discoveryPath,
    required this.hasSource,
  });

  final String slug;
  final String familyName;
  final String binaryName;
  final String buildRunner;
  final String displayName;
  final int sortRank;
  final String holonUuid;
  final String born;
  final String sourceKind;
  final String discoveryPath;
  final bool hasSource;

  String get id => slug;
  String get variant => slug.replaceFirst('gabriel-greeting-', '');

  static GabrielHolonIdentity? fromDiscovered(dynamic ref) {
    final base = DiscoveredHolonIdentity.fromDiscovered(ref);
    if (base == null) {
      return null;
    }
    final slug = base.slug;
    if (!slug.startsWith('gabriel-greeting-') ||
        slug == 'gabriel-greeting-app-swiftui' ||
        slug == 'gabriel-greeting-app-flutter') {
      return null;
    }
    return GabrielHolonIdentity(
      slug: slug,
      familyName: base.familyName,
      binaryName: base.binaryName,
      buildRunner: base.buildRunner,
      displayName: displayNameFor(slug),
      sortRank: sortRankFor(slug),
      holonUuid: base.holonUuid,
      born: '',
      sourceKind: base.sourceKind,
      discoveryPath: base.discoveryPath,
      hasSource: base.hasSource,
    );
  }

  static String displayNameFor(String slug) {
    switch (slug.replaceFirst('gabriel-greeting-', '')) {
      case 'cpp':
        return 'Gabriel (C++)';
      case 'csharp':
        return 'Gabriel (C#)';
      case 'node':
        return 'Gabriel (Node.js)';
      default:
        final variant = slug
            .replaceFirst('gabriel-greeting-', '')
            .split('-')
            .where((part) => part.trim().isNotEmpty)
            .map(_capitalize)
            .join(' ');
        return 'Gabriel ($variant)';
    }
  }

  static int sortRankFor(String slug) {
    return _sortOrder[slug] ?? 999;
  }

  static String _capitalize(String value) {
    if (value.isEmpty) {
      return value;
    }
    return '${value[0].toUpperCase()}${value.substring(1)}';
  }

  @override
  bool operator ==(Object other) {
    return other is GabrielHolonIdentity && other.slug == slug;
  }

  @override
  int get hashCode => slug.hashCode;

  static final _sortOrder = <String, int>{
    'gabriel-greeting-swift': 0,
    'gabriel-greeting-go': 1,
    'gabriel-greeting-rust': 2,
    'gabriel-greeting-zig': 3,
    'gabriel-greeting-python': 4,
    'gabriel-greeting-c': 5,
    'gabriel-greeting-cpp': 6,
    'gabriel-greeting-csharp': 7,
    'gabriel-greeting-dart': 8,
    'gabriel-greeting-java': 9,
    'gabriel-greeting-kotlin': 10,
    'gabriel-greeting-node': 11,
    'gabriel-greeting-ruby': 12,
  };
}

GabrielHolonIdentity? preferredHolon(Iterable<GabrielHolonIdentity> holons) {
  GabrielHolonIdentity? best;
  for (final holon in holons) {
    if (best == null || holon.sortRank < best.sortRank) {
      best = holon;
    }
  }
  return best;
}
