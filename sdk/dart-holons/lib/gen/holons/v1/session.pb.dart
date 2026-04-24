// This is a generated file - do not edit.
//
// Generated from holons/v1/session.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import 'session.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'session.pbenum.dart';

class SessionsRequest extends $pb.GeneratedMessage {
  factory SessionsRequest({
    $core.Iterable<SessionState>? stateFilter,
    SessionDirection? directionFilter,
    $core.bool? includeClosed,
    $core.int? limit,
    $core.String? pageToken,
  }) {
    final result = create();
    if (stateFilter != null) result.stateFilter.addAll(stateFilter);
    if (directionFilter != null) result.directionFilter = directionFilter;
    if (includeClosed != null) result.includeClosed = includeClosed;
    if (limit != null) result.limit = limit;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  SessionsRequest._();

  factory SessionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SessionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SessionsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pc<SessionState>(
        1, _omitFieldNames ? '' : 'stateFilter', $pb.PbFieldType.KE,
        valueOf: SessionState.valueOf,
        enumValues: SessionState.values,
        defaultEnumValue: SessionState.SESSION_STATE_UNSPECIFIED)
    ..aE<SessionDirection>(2, _omitFieldNames ? '' : 'directionFilter',
        enumValues: SessionDirection.values)
    ..aOB(3, _omitFieldNames ? '' : 'includeClosed')
    ..aI(4, _omitFieldNames ? '' : 'limit')
    ..aOS(5, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionsRequest copyWith(void Function(SessionsRequest) updates) =>
      super.copyWith((message) => updates(message as SessionsRequest))
          as SessionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SessionsRequest create() => SessionsRequest._();
  @$core.override
  SessionsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SessionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SessionsRequest>(create);
  static SessionsRequest? _defaultInstance;

  /// Filter by state. Empty = all non-CLOSED sessions.
  @$pb.TagNumber(1)
  $pb.PbList<SessionState> get stateFilter => $_getList(0);

  /// Filter by direction. UNSPECIFIED = both.
  @$pb.TagNumber(2)
  SessionDirection get directionFilter => $_getN(1);
  @$pb.TagNumber(2)
  set directionFilter(SessionDirection value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasDirectionFilter() => $_has(1);
  @$pb.TagNumber(2)
  void clearDirectionFilter() => $_clearField(2);

  /// Include closed sessions from the history ring buffer.
  @$pb.TagNumber(3)
  $core.bool get includeClosed => $_getBF(2);
  @$pb.TagNumber(3)
  set includeClosed($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIncludeClosed() => $_has(2);
  @$pb.TagNumber(3)
  void clearIncludeClosed() => $_clearField(3);

  /// Maximum number of sessions to return. 0 = server default (100).
  @$pb.TagNumber(4)
  $core.int get limit => $_getIZ(3);
  @$pb.TagNumber(4)
  set limit($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasLimit() => $_has(3);
  @$pb.TagNumber(4)
  void clearLimit() => $_clearField(4);

  /// Opaque continuation token from a previous response.
  /// Empty = start from the beginning.
  @$pb.TagNumber(5)
  $core.String get pageToken => $_getSZ(4);
  @$pb.TagNumber(5)
  set pageToken($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPageToken() => $_has(4);
  @$pb.TagNumber(5)
  void clearPageToken() => $_clearField(5);
}

class SessionsResponse extends $pb.GeneratedMessage {
  factory SessionsResponse({
    $core.String? slug,
    $core.Iterable<SessionInfo>? sessions,
    $core.String? nextPageToken,
    $core.int? totalCount,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    if (sessions != null) result.sessions.addAll(sessions);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    if (totalCount != null) result.totalCount = totalCount;
    return result;
  }

  SessionsResponse._();

  factory SessionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SessionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SessionsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..pPM<SessionInfo>(2, _omitFieldNames ? '' : 'sessions',
        subBuilder: SessionInfo.create)
    ..aOS(3, _omitFieldNames ? '' : 'nextPageToken')
    ..aI(4, _omitFieldNames ? '' : 'totalCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionsResponse copyWith(void Function(SessionsResponse) updates) =>
      super.copyWith((message) => updates(message as SessionsResponse))
          as SessionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SessionsResponse create() => SessionsResponse._();
  @$core.override
  SessionsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SessionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SessionsResponse>(create);
  static SessionsResponse? _defaultInstance;

  /// The holon's own slug.
  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);

  /// Matching sessions, ordered by started_at descending.
  @$pb.TagNumber(2)
  $pb.PbList<SessionInfo> get sessions => $_getList(1);

  /// Continuation token for the next page. Empty = no more results.
  @$pb.TagNumber(3)
  $core.String get nextPageToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set nextPageToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNextPageToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearNextPageToken() => $_clearField(3);

  /// Total number of sessions matching the filter (across all pages).
  @$pb.TagNumber(4)
  $core.int get totalCount => $_getIZ(3);
  @$pb.TagNumber(4)
  set totalCount($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTotalCount() => $_has(3);
  @$pb.TagNumber(4)
  void clearTotalCount() => $_clearField(4);
}

class SessionInfo extends $pb.GeneratedMessage {
  factory SessionInfo({
    $core.String? sessionId,
    $core.String? remoteSlug,
    $core.String? transport,
    $core.String? address,
    SessionDirection? direction,
    SessionState? state,
    $1.Timestamp? startedAt,
    $1.Timestamp? stateChangedAt,
    $1.Timestamp? endedAt,
    $fixnum.Int64? rpcCount,
    $1.Timestamp? lastRpcAt,
    SessionMetrics? metrics,
    $core.String? meshHost,
    $core.String? instanceUid,
  }) {
    final result = create();
    if (sessionId != null) result.sessionId = sessionId;
    if (remoteSlug != null) result.remoteSlug = remoteSlug;
    if (transport != null) result.transport = transport;
    if (address != null) result.address = address;
    if (direction != null) result.direction = direction;
    if (state != null) result.state = state;
    if (startedAt != null) result.startedAt = startedAt;
    if (stateChangedAt != null) result.stateChangedAt = stateChangedAt;
    if (endedAt != null) result.endedAt = endedAt;
    if (rpcCount != null) result.rpcCount = rpcCount;
    if (lastRpcAt != null) result.lastRpcAt = lastRpcAt;
    if (metrics != null) result.metrics = metrics;
    if (meshHost != null) result.meshHost = meshHost;
    if (instanceUid != null) result.instanceUid = instanceUid;
    return result;
  }

  SessionInfo._();

  factory SessionInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SessionInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SessionInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'remoteSlug')
    ..aOS(3, _omitFieldNames ? '' : 'transport')
    ..aOS(4, _omitFieldNames ? '' : 'address')
    ..aE<SessionDirection>(5, _omitFieldNames ? '' : 'direction',
        enumValues: SessionDirection.values)
    ..aE<SessionState>(6, _omitFieldNames ? '' : 'state',
        enumValues: SessionState.values)
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'startedAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'stateChangedAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'endedAt',
        subBuilder: $1.Timestamp.create)
    ..aInt64(10, _omitFieldNames ? '' : 'rpcCount')
    ..aOM<$1.Timestamp>(11, _omitFieldNames ? '' : 'lastRpcAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<SessionMetrics>(20, _omitFieldNames ? '' : 'metrics',
        subBuilder: SessionMetrics.create)
    ..aOS(21, _omitFieldNames ? '' : 'meshHost')
    ..aOS(22, _omitFieldNames ? '' : 'instanceUid')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionInfo copyWith(void Function(SessionInfo) updates) =>
      super.copyWith((message) => updates(message as SessionInfo))
          as SessionInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SessionInfo create() => SessionInfo._();
  @$core.override
  SessionInfo createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SessionInfo getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SessionInfo>(create);
  static SessionInfo? _defaultInstance;

  /// Unique session identifier (UUID v4).
  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  /// Slug of the remote peer. Interpretation depends on direction:
  /// the caller for INBOUND, the dialed target for OUTBOUND.
  /// "anonymous" when no x-holon-slug header was presented on inbound.
  @$pb.TagNumber(2)
  $core.String get remoteSlug => $_getSZ(1);
  @$pb.TagNumber(2)
  set remoteSlug($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRemoteSlug() => $_has(1);
  @$pb.TagNumber(2)
  void clearRemoteSlug() => $_clearField(2);

  /// Transport scheme used for this session.
  @$pb.TagNumber(3)
  $core.String get transport => $_getSZ(2);
  @$pb.TagNumber(3)
  set transport($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTransport() => $_has(2);
  @$pb.TagNumber(3)
  void clearTransport() => $_clearField(3);

  /// Concrete transport address.
  @$pb.TagNumber(4)
  $core.String get address => $_getSZ(3);
  @$pb.TagNumber(4)
  set address($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAddress() => $_has(3);
  @$pb.TagNumber(4)
  void clearAddress() => $_clearField(4);

  /// Whether this holon is the server or client side.
  @$pb.TagNumber(5)
  SessionDirection get direction => $_getN(4);
  @$pb.TagNumber(5)
  set direction(SessionDirection value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasDirection() => $_has(4);
  @$pb.TagNumber(5)
  void clearDirection() => $_clearField(5);

  /// Current lifecycle state.
  @$pb.TagNumber(6)
  SessionState get state => $_getN(5);
  @$pb.TagNumber(6)
  set state(SessionState value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasState() => $_has(5);
  @$pb.TagNumber(6)
  void clearState() => $_clearField(6);

  /// When the session was created (dial or accept).
  @$pb.TagNumber(7)
  $1.Timestamp get startedAt => $_getN(6);
  @$pb.TagNumber(7)
  set startedAt($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasStartedAt() => $_has(6);
  @$pb.TagNumber(7)
  void clearStartedAt() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensureStartedAt() => $_ensure(6);

  /// When the state last changed.
  @$pb.TagNumber(8)
  $1.Timestamp get stateChangedAt => $_getN(7);
  @$pb.TagNumber(8)
  set stateChangedAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasStateChangedAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearStateChangedAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureStateChangedAt() => $_ensure(7);

  /// When the session reached CLOSED (zero if still open).
  @$pb.TagNumber(9)
  $1.Timestamp get endedAt => $_getN(8);
  @$pb.TagNumber(9)
  set endedAt($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasEndedAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearEndedAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureEndedAt() => $_ensure(8);

  /// Number of RPCs completed in this session.
  @$pb.TagNumber(10)
  $fixnum.Int64 get rpcCount => $_getI64(9);
  @$pb.TagNumber(10)
  set rpcCount($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasRpcCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearRpcCount() => $_clearField(10);

  /// Last RPC completed timestamp (zero if none).
  @$pb.TagNumber(11)
  $1.Timestamp get lastRpcAt => $_getN(10);
  @$pb.TagNumber(11)
  set lastRpcAt($1.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasLastRpcAt() => $_has(10);
  @$pb.TagNumber(11)
  void clearLastRpcAt() => $_clearField(11);
  @$pb.TagNumber(11)
  $1.Timestamp ensureLastRpcAt() => $_ensure(10);

  /// Reserved for v2; not emitted in v1.
  @$pb.TagNumber(20)
  SessionMetrics get metrics => $_getN(11);
  @$pb.TagNumber(20)
  set metrics(SessionMetrics value) => $_setField(20, value);
  @$pb.TagNumber(20)
  $core.bool hasMetrics() => $_has(11);
  @$pb.TagNumber(20)
  void clearMetrics() => $_clearField(20);
  @$pb.TagNumber(20)
  SessionMetrics ensureMetrics() => $_ensure(11);

  /// Mesh host name (only on mTLS connections, empty otherwise).
  @$pb.TagNumber(21)
  $core.String get meshHost => $_getSZ(12);
  @$pb.TagNumber(21)
  set meshHost($core.String value) => $_setString(12, value);
  @$pb.TagNumber(21)
  $core.bool hasMeshHost() => $_has(12);
  @$pb.TagNumber(21)
  void clearMeshHost() => $_clearField(21);

  /// Owning instance UID (see INSTANCES.md). Populated from
  /// OP_INSTANCE_UID set by the parent supervisor. Empty for
  /// manually launched holons. Join key for observability signals
  /// (OBSERVABILITY.md).
  @$pb.TagNumber(22)
  $core.String get instanceUid => $_getSZ(13);
  @$pb.TagNumber(22)
  set instanceUid($core.String value) => $_setString(13, value);
  @$pb.TagNumber(22)
  $core.bool hasInstanceUid() => $_has(13);
  @$pb.TagNumber(22)
  void clearInstanceUid() => $_clearField(22);
}

/// SessionMetrics is reserved for v2; empty in v1.
/// Time is decomposed into four phases: wire_out, queue, work, wire_in.
class SessionMetrics extends $pb.GeneratedMessage {
  factory SessionMetrics({
    $fixnum.Int64? totalP50Us,
    $fixnum.Int64? totalP99Us,
    $fixnum.Int64? wireOutP50Us,
    $fixnum.Int64? wireOutP99Us,
    $fixnum.Int64? queueP50Us,
    $fixnum.Int64? queueP99Us,
    $fixnum.Int64? workP50Us,
    $fixnum.Int64? workP99Us,
    $fixnum.Int64? wireInP50Us,
    $fixnum.Int64? wireInP99Us,
    $fixnum.Int64? errorCount,
    $fixnum.Int64? bytesSent,
    $fixnum.Int64? bytesReceived,
    $core.int? inFlight,
    $fixnum.Int64? messagesSent,
    $fixnum.Int64? messagesReceived,
    $core.Iterable<$core.MapEntry<$core.String, MethodMetrics>>? methods,
  }) {
    final result = create();
    if (totalP50Us != null) result.totalP50Us = totalP50Us;
    if (totalP99Us != null) result.totalP99Us = totalP99Us;
    if (wireOutP50Us != null) result.wireOutP50Us = wireOutP50Us;
    if (wireOutP99Us != null) result.wireOutP99Us = wireOutP99Us;
    if (queueP50Us != null) result.queueP50Us = queueP50Us;
    if (queueP99Us != null) result.queueP99Us = queueP99Us;
    if (workP50Us != null) result.workP50Us = workP50Us;
    if (workP99Us != null) result.workP99Us = workP99Us;
    if (wireInP50Us != null) result.wireInP50Us = wireInP50Us;
    if (wireInP99Us != null) result.wireInP99Us = wireInP99Us;
    if (errorCount != null) result.errorCount = errorCount;
    if (bytesSent != null) result.bytesSent = bytesSent;
    if (bytesReceived != null) result.bytesReceived = bytesReceived;
    if (inFlight != null) result.inFlight = inFlight;
    if (messagesSent != null) result.messagesSent = messagesSent;
    if (messagesReceived != null) result.messagesReceived = messagesReceived;
    if (methods != null) result.methods.addEntries(methods);
    return result;
  }

  SessionMetrics._();

  factory SessionMetrics.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SessionMetrics.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SessionMetrics',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aInt64(1, _omitFieldNames ? '' : 'totalP50Us')
    ..aInt64(2, _omitFieldNames ? '' : 'totalP99Us')
    ..aInt64(3, _omitFieldNames ? '' : 'wireOutP50Us')
    ..aInt64(4, _omitFieldNames ? '' : 'wireOutP99Us')
    ..aInt64(5, _omitFieldNames ? '' : 'queueP50Us')
    ..aInt64(6, _omitFieldNames ? '' : 'queueP99Us')
    ..aInt64(7, _omitFieldNames ? '' : 'workP50Us')
    ..aInt64(8, _omitFieldNames ? '' : 'workP99Us')
    ..aInt64(9, _omitFieldNames ? '' : 'wireInP50Us')
    ..aInt64(10, _omitFieldNames ? '' : 'wireInP99Us')
    ..aInt64(11, _omitFieldNames ? '' : 'errorCount')
    ..aInt64(12, _omitFieldNames ? '' : 'bytesSent')
    ..aInt64(13, _omitFieldNames ? '' : 'bytesReceived')
    ..aI(14, _omitFieldNames ? '' : 'inFlight')
    ..aInt64(15, _omitFieldNames ? '' : 'messagesSent')
    ..aInt64(16, _omitFieldNames ? '' : 'messagesReceived')
    ..m<$core.String, MethodMetrics>(20, _omitFieldNames ? '' : 'methods',
        entryClassName: 'SessionMetrics.MethodsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: MethodMetrics.create,
        valueDefaultOrMaker: MethodMetrics.getDefault,
        packageName: const $pb.PackageName('holons.v1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionMetrics clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionMetrics copyWith(void Function(SessionMetrics) updates) =>
      super.copyWith((message) => updates(message as SessionMetrics))
          as SessionMetrics;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SessionMetrics create() => SessionMetrics._();
  @$core.override
  SessionMetrics createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SessionMetrics getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SessionMetrics>(create);
  static SessionMetrics? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get totalP50Us => $_getI64(0);
  @$pb.TagNumber(1)
  set totalP50Us($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTotalP50Us() => $_has(0);
  @$pb.TagNumber(1)
  void clearTotalP50Us() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get totalP99Us => $_getI64(1);
  @$pb.TagNumber(2)
  set totalP99Us($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTotalP99Us() => $_has(1);
  @$pb.TagNumber(2)
  void clearTotalP99Us() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get wireOutP50Us => $_getI64(2);
  @$pb.TagNumber(3)
  set wireOutP50Us($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWireOutP50Us() => $_has(2);
  @$pb.TagNumber(3)
  void clearWireOutP50Us() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get wireOutP99Us => $_getI64(3);
  @$pb.TagNumber(4)
  set wireOutP99Us($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasWireOutP99Us() => $_has(3);
  @$pb.TagNumber(4)
  void clearWireOutP99Us() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get queueP50Us => $_getI64(4);
  @$pb.TagNumber(5)
  set queueP50Us($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasQueueP50Us() => $_has(4);
  @$pb.TagNumber(5)
  void clearQueueP50Us() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get queueP99Us => $_getI64(5);
  @$pb.TagNumber(6)
  set queueP99Us($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasQueueP99Us() => $_has(5);
  @$pb.TagNumber(6)
  void clearQueueP99Us() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get workP50Us => $_getI64(6);
  @$pb.TagNumber(7)
  set workP50Us($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasWorkP50Us() => $_has(6);
  @$pb.TagNumber(7)
  void clearWorkP50Us() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get workP99Us => $_getI64(7);
  @$pb.TagNumber(8)
  set workP99Us($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasWorkP99Us() => $_has(7);
  @$pb.TagNumber(8)
  void clearWorkP99Us() => $_clearField(8);

  @$pb.TagNumber(9)
  $fixnum.Int64 get wireInP50Us => $_getI64(8);
  @$pb.TagNumber(9)
  set wireInP50Us($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasWireInP50Us() => $_has(8);
  @$pb.TagNumber(9)
  void clearWireInP50Us() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get wireInP99Us => $_getI64(9);
  @$pb.TagNumber(10)
  set wireInP99Us($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasWireInP99Us() => $_has(9);
  @$pb.TagNumber(10)
  void clearWireInP99Us() => $_clearField(10);

  @$pb.TagNumber(11)
  $fixnum.Int64 get errorCount => $_getI64(10);
  @$pb.TagNumber(11)
  set errorCount($fixnum.Int64 value) => $_setInt64(10, value);
  @$pb.TagNumber(11)
  $core.bool hasErrorCount() => $_has(10);
  @$pb.TagNumber(11)
  void clearErrorCount() => $_clearField(11);

  @$pb.TagNumber(12)
  $fixnum.Int64 get bytesSent => $_getI64(11);
  @$pb.TagNumber(12)
  set bytesSent($fixnum.Int64 value) => $_setInt64(11, value);
  @$pb.TagNumber(12)
  $core.bool hasBytesSent() => $_has(11);
  @$pb.TagNumber(12)
  void clearBytesSent() => $_clearField(12);

  @$pb.TagNumber(13)
  $fixnum.Int64 get bytesReceived => $_getI64(12);
  @$pb.TagNumber(13)
  set bytesReceived($fixnum.Int64 value) => $_setInt64(12, value);
  @$pb.TagNumber(13)
  $core.bool hasBytesReceived() => $_has(12);
  @$pb.TagNumber(13)
  void clearBytesReceived() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.int get inFlight => $_getIZ(13);
  @$pb.TagNumber(14)
  set inFlight($core.int value) => $_setSignedInt32(13, value);
  @$pb.TagNumber(14)
  $core.bool hasInFlight() => $_has(13);
  @$pb.TagNumber(14)
  void clearInFlight() => $_clearField(14);

  @$pb.TagNumber(15)
  $fixnum.Int64 get messagesSent => $_getI64(14);
  @$pb.TagNumber(15)
  set messagesSent($fixnum.Int64 value) => $_setInt64(14, value);
  @$pb.TagNumber(15)
  $core.bool hasMessagesSent() => $_has(14);
  @$pb.TagNumber(15)
  void clearMessagesSent() => $_clearField(15);

  @$pb.TagNumber(16)
  $fixnum.Int64 get messagesReceived => $_getI64(15);
  @$pb.TagNumber(16)
  set messagesReceived($fixnum.Int64 value) => $_setInt64(15, value);
  @$pb.TagNumber(16)
  $core.bool hasMessagesReceived() => $_has(15);
  @$pb.TagNumber(16)
  void clearMessagesReceived() => $_clearField(16);

  @$pb.TagNumber(20)
  $pb.PbMap<$core.String, MethodMetrics> get methods => $_getMap(16);
}

class MethodMetrics extends $pb.GeneratedMessage {
  factory MethodMetrics({
    $fixnum.Int64? callCount,
    $fixnum.Int64? errorCount,
    $core.int? inFlight,
    $fixnum.Int64? totalP50Us,
    $fixnum.Int64? totalP99Us,
    $fixnum.Int64? wireOutP50Us,
    $fixnum.Int64? wireOutP99Us,
    $fixnum.Int64? queueP50Us,
    $fixnum.Int64? queueP99Us,
    $fixnum.Int64? workP50Us,
    $fixnum.Int64? workP99Us,
    $fixnum.Int64? wireInP50Us,
    $fixnum.Int64? wireInP99Us,
  }) {
    final result = create();
    if (callCount != null) result.callCount = callCount;
    if (errorCount != null) result.errorCount = errorCount;
    if (inFlight != null) result.inFlight = inFlight;
    if (totalP50Us != null) result.totalP50Us = totalP50Us;
    if (totalP99Us != null) result.totalP99Us = totalP99Us;
    if (wireOutP50Us != null) result.wireOutP50Us = wireOutP50Us;
    if (wireOutP99Us != null) result.wireOutP99Us = wireOutP99Us;
    if (queueP50Us != null) result.queueP50Us = queueP50Us;
    if (queueP99Us != null) result.queueP99Us = queueP99Us;
    if (workP50Us != null) result.workP50Us = workP50Us;
    if (workP99Us != null) result.workP99Us = workP99Us;
    if (wireInP50Us != null) result.wireInP50Us = wireInP50Us;
    if (wireInP99Us != null) result.wireInP99Us = wireInP99Us;
    return result;
  }

  MethodMetrics._();

  factory MethodMetrics.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MethodMetrics.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MethodMetrics',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aInt64(1, _omitFieldNames ? '' : 'callCount')
    ..aInt64(2, _omitFieldNames ? '' : 'errorCount')
    ..aI(3, _omitFieldNames ? '' : 'inFlight')
    ..aInt64(10, _omitFieldNames ? '' : 'totalP50Us')
    ..aInt64(11, _omitFieldNames ? '' : 'totalP99Us')
    ..aInt64(12, _omitFieldNames ? '' : 'wireOutP50Us')
    ..aInt64(13, _omitFieldNames ? '' : 'wireOutP99Us')
    ..aInt64(14, _omitFieldNames ? '' : 'queueP50Us')
    ..aInt64(15, _omitFieldNames ? '' : 'queueP99Us')
    ..aInt64(16, _omitFieldNames ? '' : 'workP50Us')
    ..aInt64(17, _omitFieldNames ? '' : 'workP99Us')
    ..aInt64(18, _omitFieldNames ? '' : 'wireInP50Us')
    ..aInt64(19, _omitFieldNames ? '' : 'wireInP99Us')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MethodMetrics clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MethodMetrics copyWith(void Function(MethodMetrics) updates) =>
      super.copyWith((message) => updates(message as MethodMetrics))
          as MethodMetrics;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MethodMetrics create() => MethodMetrics._();
  @$core.override
  MethodMetrics createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MethodMetrics getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MethodMetrics>(create);
  static MethodMetrics? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get callCount => $_getI64(0);
  @$pb.TagNumber(1)
  set callCount($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCallCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallCount() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get errorCount => $_getI64(1);
  @$pb.TagNumber(2)
  set errorCount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasErrorCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearErrorCount() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get inFlight => $_getIZ(2);
  @$pb.TagNumber(3)
  set inFlight($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasInFlight() => $_has(2);
  @$pb.TagNumber(3)
  void clearInFlight() => $_clearField(3);

  @$pb.TagNumber(10)
  $fixnum.Int64 get totalP50Us => $_getI64(3);
  @$pb.TagNumber(10)
  set totalP50Us($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(10)
  $core.bool hasTotalP50Us() => $_has(3);
  @$pb.TagNumber(10)
  void clearTotalP50Us() => $_clearField(10);

  @$pb.TagNumber(11)
  $fixnum.Int64 get totalP99Us => $_getI64(4);
  @$pb.TagNumber(11)
  set totalP99Us($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(11)
  $core.bool hasTotalP99Us() => $_has(4);
  @$pb.TagNumber(11)
  void clearTotalP99Us() => $_clearField(11);

  @$pb.TagNumber(12)
  $fixnum.Int64 get wireOutP50Us => $_getI64(5);
  @$pb.TagNumber(12)
  set wireOutP50Us($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(12)
  $core.bool hasWireOutP50Us() => $_has(5);
  @$pb.TagNumber(12)
  void clearWireOutP50Us() => $_clearField(12);

  @$pb.TagNumber(13)
  $fixnum.Int64 get wireOutP99Us => $_getI64(6);
  @$pb.TagNumber(13)
  set wireOutP99Us($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(13)
  $core.bool hasWireOutP99Us() => $_has(6);
  @$pb.TagNumber(13)
  void clearWireOutP99Us() => $_clearField(13);

  @$pb.TagNumber(14)
  $fixnum.Int64 get queueP50Us => $_getI64(7);
  @$pb.TagNumber(14)
  set queueP50Us($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(14)
  $core.bool hasQueueP50Us() => $_has(7);
  @$pb.TagNumber(14)
  void clearQueueP50Us() => $_clearField(14);

  @$pb.TagNumber(15)
  $fixnum.Int64 get queueP99Us => $_getI64(8);
  @$pb.TagNumber(15)
  set queueP99Us($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(15)
  $core.bool hasQueueP99Us() => $_has(8);
  @$pb.TagNumber(15)
  void clearQueueP99Us() => $_clearField(15);

  @$pb.TagNumber(16)
  $fixnum.Int64 get workP50Us => $_getI64(9);
  @$pb.TagNumber(16)
  set workP50Us($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(16)
  $core.bool hasWorkP50Us() => $_has(9);
  @$pb.TagNumber(16)
  void clearWorkP50Us() => $_clearField(16);

  @$pb.TagNumber(17)
  $fixnum.Int64 get workP99Us => $_getI64(10);
  @$pb.TagNumber(17)
  set workP99Us($fixnum.Int64 value) => $_setInt64(10, value);
  @$pb.TagNumber(17)
  $core.bool hasWorkP99Us() => $_has(10);
  @$pb.TagNumber(17)
  void clearWorkP99Us() => $_clearField(17);

  @$pb.TagNumber(18)
  $fixnum.Int64 get wireInP50Us => $_getI64(11);
  @$pb.TagNumber(18)
  set wireInP50Us($fixnum.Int64 value) => $_setInt64(11, value);
  @$pb.TagNumber(18)
  $core.bool hasWireInP50Us() => $_has(11);
  @$pb.TagNumber(18)
  void clearWireInP50Us() => $_clearField(18);

  @$pb.TagNumber(19)
  $fixnum.Int64 get wireInP99Us => $_getI64(12);
  @$pb.TagNumber(19)
  set wireInP99Us($fixnum.Int64 value) => $_setInt64(12, value);
  @$pb.TagNumber(19)
  $core.bool hasWireInP99Us() => $_has(12);
  @$pb.TagNumber(19)
  void clearWireInP99Us() => $_clearField(19);
}

class WatchSessionsRequest extends $pb.GeneratedMessage {
  factory WatchSessionsRequest({
    $core.Iterable<SessionState>? stateFilter,
    SessionDirection? directionFilter,
    $core.bool? sendInitialSnapshot,
  }) {
    final result = create();
    if (stateFilter != null) result.stateFilter.addAll(stateFilter);
    if (directionFilter != null) result.directionFilter = directionFilter;
    if (sendInitialSnapshot != null)
      result.sendInitialSnapshot = sendInitialSnapshot;
    return result;
  }

  WatchSessionsRequest._();

  factory WatchSessionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory WatchSessionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchSessionsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pc<SessionState>(
        1, _omitFieldNames ? '' : 'stateFilter', $pb.PbFieldType.KE,
        valueOf: SessionState.valueOf,
        enumValues: SessionState.values,
        defaultEnumValue: SessionState.SESSION_STATE_UNSPECIFIED)
    ..aE<SessionDirection>(2, _omitFieldNames ? '' : 'directionFilter',
        enumValues: SessionDirection.values)
    ..aOB(3, _omitFieldNames ? '' : 'sendInitialSnapshot')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchSessionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchSessionsRequest copyWith(void Function(WatchSessionsRequest) updates) =>
      super.copyWith((message) => updates(message as WatchSessionsRequest))
          as WatchSessionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static WatchSessionsRequest create() => WatchSessionsRequest._();
  @$core.override
  WatchSessionsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static WatchSessionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WatchSessionsRequest>(create);
  static WatchSessionsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SessionState> get stateFilter => $_getList(0);

  @$pb.TagNumber(2)
  SessionDirection get directionFilter => $_getN(1);
  @$pb.TagNumber(2)
  set directionFilter(SessionDirection value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasDirectionFilter() => $_has(1);
  @$pb.TagNumber(2)
  void clearDirectionFilter() => $_clearField(2);

  /// If true, a SessionEvent with kind=SNAPSHOT is emitted for every
  /// currently matching session before the stream transitions to live
  /// events. Useful for clients that need a consistent view.
  @$pb.TagNumber(3)
  $core.bool get sendInitialSnapshot => $_getBF(2);
  @$pb.TagNumber(3)
  set sendInitialSnapshot($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSendInitialSnapshot() => $_has(2);
  @$pb.TagNumber(3)
  void clearSendInitialSnapshot() => $_clearField(3);
}

class SessionEvent extends $pb.GeneratedMessage {
  factory SessionEvent({
    $1.Timestamp? ts,
    SessionEventKind? kind,
    SessionInfo? session,
    SessionState? previousState,
  }) {
    final result = create();
    if (ts != null) result.ts = ts;
    if (kind != null) result.kind = kind;
    if (session != null) result.session = session;
    if (previousState != null) result.previousState = previousState;
    return result;
  }

  SessionEvent._();

  factory SessionEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SessionEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SessionEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOM<$1.Timestamp>(1, _omitFieldNames ? '' : 'ts',
        subBuilder: $1.Timestamp.create)
    ..aE<SessionEventKind>(2, _omitFieldNames ? '' : 'kind',
        enumValues: SessionEventKind.values)
    ..aOM<SessionInfo>(3, _omitFieldNames ? '' : 'session',
        subBuilder: SessionInfo.create)
    ..aE<SessionState>(4, _omitFieldNames ? '' : 'previousState',
        enumValues: SessionState.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionEvent copyWith(void Function(SessionEvent) updates) =>
      super.copyWith((message) => updates(message as SessionEvent))
          as SessionEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SessionEvent create() => SessionEvent._();
  @$core.override
  SessionEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SessionEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SessionEvent>(create);
  static SessionEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $1.Timestamp get ts => $_getN(0);
  @$pb.TagNumber(1)
  set ts($1.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTs() => $_has(0);
  @$pb.TagNumber(1)
  void clearTs() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.Timestamp ensureTs() => $_ensure(0);

  @$pb.TagNumber(2)
  SessionEventKind get kind => $_getN(1);
  @$pb.TagNumber(2)
  set kind(SessionEventKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);

  @$pb.TagNumber(3)
  SessionInfo get session => $_getN(2);
  @$pb.TagNumber(3)
  set session(SessionInfo value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSession() => $_has(2);
  @$pb.TagNumber(3)
  void clearSession() => $_clearField(3);
  @$pb.TagNumber(3)
  SessionInfo ensureSession() => $_ensure(2);

  /// For STATE_CHANGED: the previous state. Zero for other kinds.
  @$pb.TagNumber(4)
  SessionState get previousState => $_getN(3);
  @$pb.TagNumber(4)
  set previousState(SessionState value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPreviousState() => $_has(3);
  @$pb.TagNumber(4)
  void clearPreviousState() => $_clearField(4);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
