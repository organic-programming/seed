import 'discover.dart' as discover;
import 'discovery_types.dart' as discovery_types;

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

  final T? Function(discovery_types.HolonRef discovered) fromDiscovered;
  final String Function(T holon) slugOf;
  final int Function(T holon)? sortRankOf;
  final String Function(T holon)? displayNameOf;

  @override
  Future<List<T>> list() async {
    final result = discover.Discover(
      discovery_types.LOCAL,
      null,
      null,
      discovery_types.ALL,
      discovery_types.NO_LIMIT,
      discovery_types.NO_TIMEOUT,
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

    final values = deduped.values.toList();
    values.sort((left, right) {
      final leftRank = sortRankOf?.call(left) ?? 999;
      final rightRank = sortRankOf?.call(right) ?? 999;
      if (leftRank != rightRank) {
        return leftRank.compareTo(rightRank);
      }
      final leftTitle = displayNameOf?.call(left) ?? slugOf(left);
      final rightTitle = displayNameOf?.call(right) ?? slugOf(right);
      return leftTitle.toLowerCase().compareTo(rightTitle.toLowerCase());
    });
    return values;
  }
}

@Deprecated('Use Holons<T>')
typedef HolonCatalog<T> = Holons<T>;

@Deprecated('Use BundledHolons<T>')
typedef DesktopHolonCatalog<T> = BundledHolons<T>;
