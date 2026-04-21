import 'package:collection/collection.dart';
import 'package:holons/holons.dart' as holons;

class HolonRef {
  const HolonRef({
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
}

abstract class Holons<T> {
  Future<List<T>> list();

  @Deprecated('Use list()')
  Future<List<T>> discover() => list();
}

class BundledHolons<T> extends Holons<T> {
  BundledHolons({
    required this.fromDiscovered,
    required this.slugOf,
    this.sortRankOf,
    this.displayNameOf,
  });

  final T? Function(dynamic discovered) fromDiscovered;
  final String Function(T holon) slugOf;
  final int Function(T holon)? sortRankOf;
  final String Function(T holon)? displayNameOf;

  @override
  Future<List<T>> list() async {
    final result = holons.Discover(
      holons.LOCAL,
      null,
      null,
      holons.ALL,
      holons.NO_LIMIT,
      holons.NO_TIMEOUT,
    );

    if (result.error != null && result.error!.isNotEmpty) {
      throw StateError(result.error!);
    }

    final deduped = <String, T>{};
    for (final ref in result.found) {
      final holon = fromDiscovered(ref);
      if (holon == null) {
        continue;
      }
      deduped.putIfAbsent(slugOf(holon), () => holon);
    }

    return deduped.values.sorted((left, right) {
      final leftRank = sortRankOf?.call(left) ?? 999;
      final rightRank = sortRankOf?.call(right) ?? 999;
      if (leftRank != rightRank) {
        return leftRank.compareTo(rightRank);
      }
      final leftTitle = displayNameOf?.call(left) ?? slugOf(left);
      final rightTitle = displayNameOf?.call(right) ?? slugOf(right);
      return leftTitle.toLowerCase().compareTo(rightTitle.toLowerCase());
    });
  }
}

@Deprecated('Use Holons<T>')
typedef HolonCatalog<T> = Holons<T>;

@Deprecated('Use BundledHolons<T>')
typedef DesktopHolonCatalog<T> = BundledHolons<T>;
