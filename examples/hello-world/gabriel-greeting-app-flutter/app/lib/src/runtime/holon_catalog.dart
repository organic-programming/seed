import 'package:collection/collection.dart';
import 'package:holons/holons.dart' as holons;

import '../model/app_model.dart';

abstract interface class HolonCatalog {
  Future<List<GabrielHolonIdentity>> discover();
}

class DesktopHolonCatalog implements HolonCatalog {
  @override
  Future<List<GabrielHolonIdentity>> discover() async {
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

    final deduped = <String, GabrielHolonIdentity>{};
    for (final ref in result.found) {
      final holon = GabrielHolonIdentity.fromDiscovered(ref);
      if (holon == null) {
        continue;
      }
      deduped.putIfAbsent(holon.slug, () => holon);
    }

    return deduped.values.sorted((left, right) {
      if (left.sortRank != right.sortRank) {
        return left.sortRank.compareTo(right.sortRank);
      }
      return left.displayName.toLowerCase().compareTo(
        right.displayName.toLowerCase(),
      );
    });
  }
}
