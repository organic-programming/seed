const int LOCAL = 0;
const int PROXY = 1;
const int DELEGATED = 2;

const int SIBLINGS = 0x01;
const int CWD = 0x02;
const int SOURCE = 0x04;
const int BUILT = 0x08;
const int INSTALLED = 0x10;
const int CACHED = 0x20;
const int ALL = 0x3F;

const int NO_LIMIT = 0;
const int NO_TIMEOUT = 0;

class IdentityInfo {
  final String givenName;
  final String familyName;
  final String motto;
  final List<String> aliases;

  const IdentityInfo({
    this.givenName = '',
    this.familyName = '',
    this.motto = '',
    this.aliases = const <String>[],
  });

  factory IdentityInfo.fromJson(Map<String, dynamic> json) {
    return IdentityInfo(
      givenName: _readString(json, 'given_name', fallbackKey: 'givenName'),
      familyName: _readString(json, 'family_name', fallbackKey: 'familyName'),
      motto: _readString(json, 'motto'),
      aliases: _readStringList(json['aliases']),
    );
  }

  Map<String, Object?> toJson() => <String, Object?>{
        'given_name': givenName,
        'family_name': familyName,
        'motto': motto,
        'aliases': aliases,
      };
}

class HolonInfo {
  final String slug;
  final String uuid;
  final IdentityInfo identity;
  final String lang;
  final String runner;
  final String status;
  final String kind;
  final String transport;
  final String entrypoint;
  final List<String> architectures;
  final bool hasDist;
  final bool hasSource;

  const HolonInfo({
    this.slug = '',
    this.uuid = '',
    this.identity = const IdentityInfo(),
    this.lang = '',
    this.runner = '',
    this.status = '',
    this.kind = '',
    this.transport = '',
    this.entrypoint = '',
    this.architectures = const <String>[],
    this.hasDist = false,
    this.hasSource = false,
  });

  factory HolonInfo.fromJson(Map<String, dynamic> json) {
    final identityJson = json['identity'];
    return HolonInfo(
      slug: _readString(json, 'slug'),
      uuid: _readString(json, 'uuid'),
      identity: identityJson is Map<String, dynamic>
          ? IdentityInfo.fromJson(identityJson)
          : const IdentityInfo(),
      lang: _readString(json, 'lang'),
      runner: _readString(json, 'runner'),
      status: _readString(json, 'status'),
      kind: _readString(json, 'kind'),
      transport: _readString(json, 'transport'),
      entrypoint: _readString(json, 'entrypoint'),
      architectures: _readStringList(json['architectures']),
      hasDist: _readBool(json, 'has_dist', fallbackKey: 'hasDist'),
      hasSource: _readBool(json, 'has_source', fallbackKey: 'hasSource'),
    );
  }

  Map<String, Object?> toJson() => <String, Object?>{
        'slug': slug,
        'uuid': uuid,
        'identity': identity.toJson(),
        'lang': lang,
        'runner': runner,
        'status': status,
        'kind': kind,
        'transport': transport,
        'entrypoint': entrypoint,
        'architectures': architectures,
        'has_dist': hasDist,
        'has_source': hasSource,
      };
}

class HolonRef {
  final String url;
  final HolonInfo? info;
  final String? error;

  const HolonRef({
    required this.url,
    this.info,
    this.error,
  });

  factory HolonRef.fromJson(Map<String, dynamic> json) {
    final infoJson = json['info'];
    return HolonRef(
      url: _readString(json, 'url'),
      info: infoJson is Map<String, dynamic>
          ? HolonInfo.fromJson(infoJson)
          : null,
      error: _readNullableString(json, 'error'),
    );
  }

  Map<String, Object?> toJson() => <String, Object?>{
        'url': url,
        'info': info?.toJson(),
        'error': error,
      };
}

class DiscoverResult {
  final List<HolonRef> found;
  final String? error;

  const DiscoverResult({
    this.found = const <HolonRef>[],
    this.error,
  });

  factory DiscoverResult.fromJson(Map<String, dynamic> json) {
    final foundJson = json['found'];
    return DiscoverResult(
      found: foundJson is List
          ? foundJson
              .whereType<Map>()
              .map((value) => HolonRef.fromJson(
                    value.map(
                      (key, dynamic value) => MapEntry(key.toString(), value),
                    ),
                  ))
              .toList()
          : const <HolonRef>[],
      error: _readNullableString(json, 'error'),
    );
  }

  Map<String, Object?> toJson() => <String, Object?>{
        'found': found.map((ref) => ref.toJson()).toList(),
        'error': error,
      };
}

class ResolveResult {
  final HolonRef? ref;
  final String? error;

  const ResolveResult({
    this.ref,
    this.error,
  });
}

class ConnectResult {
  final dynamic channel;
  final String uid;
  final HolonRef? origin;
  final String? error;

  const ConnectResult({
    this.channel,
    this.uid = '',
    this.origin,
    this.error,
  });
}

String _readString(
  Map<String, dynamic> json,
  String key, {
  String? fallbackKey,
}) {
  final value = json[key] ?? (fallbackKey == null ? null : json[fallbackKey]);
  return value is String ? value.trim() : '';
}

String? _readNullableString(
  Map<String, dynamic> json,
  String key, {
  String? fallbackKey,
}) {
  final value = json[key] ?? (fallbackKey == null ? null : json[fallbackKey]);
  if (value is! String) {
    return null;
  }
  final trimmed = value.trim();
  return trimmed.isEmpty ? null : trimmed;
}

bool _readBool(
  Map<String, dynamic> json,
  String key, {
  String? fallbackKey,
}) {
  final value = json[key] ?? (fallbackKey == null ? null : json[fallbackKey]);
  return value == true;
}

List<String> _readStringList(dynamic value) {
  if (value is! List) {
    return const <String>[];
  }
  return value.whereType<String>().map((item) => item.trim()).toList();
}
