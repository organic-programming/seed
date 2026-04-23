// This is a generated file - do not edit.
//
// Generated from holons/v1/instance.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class ListInstancesRequest extends $pb.GeneratedMessage {
  factory ListInstancesRequest({
    $core.Iterable<$core.String>? slugs,
    $core.bool? includeStale,
  }) {
    final result = create();
    if (slugs != null) result.slugs.addAll(slugs);
    if (includeStale != null) result.includeStale = includeStale;
    return result;
  }

  ListInstancesRequest._();

  factory ListInstancesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListInstancesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListInstancesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'slugs')
    ..aOB(2, _omitFieldNames ? '' : 'includeStale')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInstancesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInstancesRequest copyWith(void Function(ListInstancesRequest) updates) =>
      super.copyWith((message) => updates(message as ListInstancesRequest))
          as ListInstancesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListInstancesRequest create() => ListInstancesRequest._();
  @$core.override
  ListInstancesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListInstancesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListInstancesRequest>(create);
  static ListInstancesRequest? _defaultInstance;

  /// Filter by slug. Empty = all slugs.
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get slugs => $_getList(0);

  /// Include instances whose PID liveness check failed (stale entries).
  @$pb.TagNumber(2)
  $core.bool get includeStale => $_getBF(1);
  @$pb.TagNumber(2)
  set includeStale($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIncludeStale() => $_has(1);
  @$pb.TagNumber(2)
  void clearIncludeStale() => $_clearField(2);
}

class ListInstancesResponse extends $pb.GeneratedMessage {
  factory ListInstancesResponse({
    $core.Iterable<InstanceInfo>? instances,
  }) {
    final result = create();
    if (instances != null) result.instances.addAll(instances);
    return result;
  }

  ListInstancesResponse._();

  factory ListInstancesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListInstancesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListInstancesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPM<InstanceInfo>(1, _omitFieldNames ? '' : 'instances',
        subBuilder: InstanceInfo.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInstancesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInstancesResponse copyWith(
          void Function(ListInstancesResponse) updates) =>
      super.copyWith((message) => updates(message as ListInstancesResponse))
          as ListInstancesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListInstancesResponse create() => ListInstancesResponse._();
  @$core.override
  ListInstancesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListInstancesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListInstancesResponse>(create);
  static ListInstancesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<InstanceInfo> get instances => $_getList(0);
}

class GetInstanceRequest extends $pb.GeneratedMessage {
  factory GetInstanceRequest({
    $core.String? uid,
  }) {
    final result = create();
    if (uid != null) result.uid = uid;
    return result;
  }

  GetInstanceRequest._();

  factory GetInstanceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetInstanceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetInstanceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'uid')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetInstanceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetInstanceRequest copyWith(void Function(GetInstanceRequest) updates) =>
      super.copyWith((message) => updates(message as GetInstanceRequest))
          as GetInstanceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetInstanceRequest create() => GetInstanceRequest._();
  @$core.override
  GetInstanceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetInstanceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetInstanceRequest>(create);
  static GetInstanceRequest? _defaultInstance;

  /// Full or prefix UID.
  @$pb.TagNumber(1)
  $core.String get uid => $_getSZ(0);
  @$pb.TagNumber(1)
  set uid($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUid() => $_has(0);
  @$pb.TagNumber(1)
  void clearUid() => $_clearField(1);
}

class InstanceInfo extends $pb.GeneratedMessage {
  factory InstanceInfo({
    $core.String? slug,
    $core.String? uid,
    $core.int? pid,
    $1.Timestamp? startedAt,
    $core.String? mode,
    $core.String? transport,
    $core.String? address,
    $core.String? metricsAddr,
    $core.String? logPath,
    $core.bool? default_10,
    $core.bool? stale,
    $core.String? organismUid,
    $core.String? organismSlug,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    if (uid != null) result.uid = uid;
    if (pid != null) result.pid = pid;
    if (startedAt != null) result.startedAt = startedAt;
    if (mode != null) result.mode = mode;
    if (transport != null) result.transport = transport;
    if (address != null) result.address = address;
    if (metricsAddr != null) result.metricsAddr = metricsAddr;
    if (logPath != null) result.logPath = logPath;
    if (default_10 != null) result.default_10 = default_10;
    if (stale != null) result.stale = stale;
    if (organismUid != null) result.organismUid = organismUid;
    if (organismSlug != null) result.organismSlug = organismSlug;
    return result;
  }

  InstanceInfo._();

  factory InstanceInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory InstanceInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InstanceInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..aOS(2, _omitFieldNames ? '' : 'uid')
    ..aI(3, _omitFieldNames ? '' : 'pid')
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'startedAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(5, _omitFieldNames ? '' : 'mode')
    ..aOS(6, _omitFieldNames ? '' : 'transport')
    ..aOS(7, _omitFieldNames ? '' : 'address')
    ..aOS(8, _omitFieldNames ? '' : 'metricsAddr')
    ..aOS(9, _omitFieldNames ? '' : 'logPath')
    ..aOB(10, _omitFieldNames ? '' : 'default')
    ..aOB(11, _omitFieldNames ? '' : 'stale')
    ..aOS(12, _omitFieldNames ? '' : 'organismUid')
    ..aOS(13, _omitFieldNames ? '' : 'organismSlug')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InstanceInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InstanceInfo copyWith(void Function(InstanceInfo) updates) =>
      super.copyWith((message) => updates(message as InstanceInfo))
          as InstanceInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static InstanceInfo create() => InstanceInfo._();
  @$core.override
  InstanceInfo createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static InstanceInfo getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<InstanceInfo>(create);
  static InstanceInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get uid => $_getSZ(1);
  @$pb.TagNumber(2)
  set uid($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUid() => $_has(1);
  @$pb.TagNumber(2)
  void clearUid() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get pid => $_getIZ(2);
  @$pb.TagNumber(3)
  set pid($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPid() => $_has(2);
  @$pb.TagNumber(3)
  void clearPid() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Timestamp get startedAt => $_getN(3);
  @$pb.TagNumber(4)
  set startedAt($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasStartedAt() => $_has(3);
  @$pb.TagNumber(4)
  void clearStartedAt() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureStartedAt() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get mode => $_getSZ(4);
  @$pb.TagNumber(5)
  set mode($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMode() => $_has(4);
  @$pb.TagNumber(5)
  void clearMode() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get transport => $_getSZ(5);
  @$pb.TagNumber(6)
  set transport($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasTransport() => $_has(5);
  @$pb.TagNumber(6)
  void clearTransport() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get address => $_getSZ(6);
  @$pb.TagNumber(7)
  set address($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasAddress() => $_has(6);
  @$pb.TagNumber(7)
  void clearAddress() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get metricsAddr => $_getSZ(7);
  @$pb.TagNumber(8)
  set metricsAddr($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasMetricsAddr() => $_has(7);
  @$pb.TagNumber(8)
  void clearMetricsAddr() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get logPath => $_getSZ(8);
  @$pb.TagNumber(9)
  set logPath($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasLogPath() => $_has(8);
  @$pb.TagNumber(9)
  void clearLogPath() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.bool get default_10 => $_getBF(9);
  @$pb.TagNumber(10)
  set default_10($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasDefault_10() => $_has(9);
  @$pb.TagNumber(10)
  void clearDefault_10() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.bool get stale => $_getBF(10);
  @$pb.TagNumber(11)
  set stale($core.bool value) => $_setBool(10, value);
  @$pb.TagNumber(11)
  $core.bool hasStale() => $_has(10);
  @$pb.TagNumber(11)
  void clearStale() => $_clearField(11);

  /// Organism membership, when set (see INSTANCES.md §Organism Hierarchy).
  @$pb.TagNumber(12)
  $core.String get organismUid => $_getSZ(11);
  @$pb.TagNumber(12)
  set organismUid($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasOrganismUid() => $_has(11);
  @$pb.TagNumber(12)
  void clearOrganismUid() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.String get organismSlug => $_getSZ(12);
  @$pb.TagNumber(13)
  set organismSlug($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasOrganismSlug() => $_has(12);
  @$pb.TagNumber(13)
  void clearOrganismSlug() => $_clearField(13);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
