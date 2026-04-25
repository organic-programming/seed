// This is a generated file - do not edit.
//
// Generated from holons/v1/observability.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/duration.pb.dart'
    as $1;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $2;

import 'observability.pbenum.dart';
import 'session.pb.dart' as $3;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'observability.pbenum.dart';

class LogsRequest extends $pb.GeneratedMessage {
  factory LogsRequest({
    LogLevel? minLevel,
    $core.Iterable<$core.String>? sessionIds,
    $core.Iterable<$core.String>? rpcMethods,
    $1.Duration? since,
    $core.bool? follow,
  }) {
    final result = create();
    if (minLevel != null) result.minLevel = minLevel;
    if (sessionIds != null) result.sessionIds.addAll(sessionIds);
    if (rpcMethods != null) result.rpcMethods.addAll(rpcMethods);
    if (since != null) result.since = since;
    if (follow != null) result.follow = follow;
    return result;
  }

  LogsRequest._();

  factory LogsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LogsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LogsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aE<LogLevel>(1, _omitFieldNames ? '' : 'minLevel',
        enumValues: LogLevel.values)
    ..pPS(2, _omitFieldNames ? '' : 'sessionIds')
    ..pPS(3, _omitFieldNames ? '' : 'rpcMethods')
    ..aOM<$1.Duration>(4, _omitFieldNames ? '' : 'since',
        subBuilder: $1.Duration.create)
    ..aOB(5, _omitFieldNames ? '' : 'follow')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogsRequest copyWith(void Function(LogsRequest) updates) =>
      super.copyWith((message) => updates(message as LogsRequest))
          as LogsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LogsRequest create() => LogsRequest._();
  @$core.override
  LogsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LogsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LogsRequest>(create);
  static LogsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  LogLevel get minLevel => $_getN(0);
  @$pb.TagNumber(1)
  set minLevel(LogLevel value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMinLevel() => $_has(0);
  @$pb.TagNumber(1)
  void clearMinLevel() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get sessionIds => $_getList(1);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get rpcMethods => $_getList(2);

  @$pb.TagNumber(4)
  $1.Duration get since => $_getN(3);
  @$pb.TagNumber(4)
  set since($1.Duration value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSince() => $_has(3);
  @$pb.TagNumber(4)
  void clearSince() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Duration ensureSince() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.bool get follow => $_getBF(4);
  @$pb.TagNumber(5)
  set follow($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasFollow() => $_has(4);
  @$pb.TagNumber(5)
  void clearFollow() => $_clearField(5);
}

class LogEntry extends $pb.GeneratedMessage {
  factory LogEntry({
    $2.Timestamp? ts,
    LogLevel? level,
    $core.String? slug,
    $core.String? instanceUid,
    $core.String? sessionId,
    $core.String? rpcMethod,
    $core.String? message,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? fields,
    $core.String? caller,
    $core.Iterable<ChainHop>? chain,
  }) {
    final result = create();
    if (ts != null) result.ts = ts;
    if (level != null) result.level = level;
    if (slug != null) result.slug = slug;
    if (instanceUid != null) result.instanceUid = instanceUid;
    if (sessionId != null) result.sessionId = sessionId;
    if (rpcMethod != null) result.rpcMethod = rpcMethod;
    if (message != null) result.message = message;
    if (fields != null) result.fields.addEntries(fields);
    if (caller != null) result.caller = caller;
    if (chain != null) result.chain.addAll(chain);
    return result;
  }

  LogEntry._();

  factory LogEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LogEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LogEntry',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOM<$2.Timestamp>(1, _omitFieldNames ? '' : 'ts',
        subBuilder: $2.Timestamp.create)
    ..aE<LogLevel>(2, _omitFieldNames ? '' : 'level',
        enumValues: LogLevel.values)
    ..aOS(3, _omitFieldNames ? '' : 'slug')
    ..aOS(4, _omitFieldNames ? '' : 'instanceUid')
    ..aOS(5, _omitFieldNames ? '' : 'sessionId')
    ..aOS(6, _omitFieldNames ? '' : 'rpcMethod')
    ..aOS(7, _omitFieldNames ? '' : 'message')
    ..m<$core.String, $core.String>(8, _omitFieldNames ? '' : 'fields',
        entryClassName: 'LogEntry.FieldsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('holons.v1'))
    ..aOS(9, _omitFieldNames ? '' : 'caller')
    ..pPM<ChainHop>(10, _omitFieldNames ? '' : 'chain',
        subBuilder: ChainHop.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogEntry copyWith(void Function(LogEntry) updates) =>
      super.copyWith((message) => updates(message as LogEntry)) as LogEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LogEntry create() => LogEntry._();
  @$core.override
  LogEntry createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LogEntry getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<LogEntry>(create);
  static LogEntry? _defaultInstance;

  @$pb.TagNumber(1)
  $2.Timestamp get ts => $_getN(0);
  @$pb.TagNumber(1)
  set ts($2.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTs() => $_has(0);
  @$pb.TagNumber(1)
  void clearTs() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.Timestamp ensureTs() => $_ensure(0);

  @$pb.TagNumber(2)
  LogLevel get level => $_getN(1);
  @$pb.TagNumber(2)
  set level(LogLevel value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasLevel() => $_has(1);
  @$pb.TagNumber(2)
  void clearLevel() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get slug => $_getSZ(2);
  @$pb.TagNumber(3)
  set slug($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSlug() => $_has(2);
  @$pb.TagNumber(3)
  void clearSlug() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get instanceUid => $_getSZ(3);
  @$pb.TagNumber(4)
  set instanceUid($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasInstanceUid() => $_has(3);
  @$pb.TagNumber(4)
  void clearInstanceUid() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get sessionId => $_getSZ(4);
  @$pb.TagNumber(5)
  set sessionId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSessionId() => $_has(4);
  @$pb.TagNumber(5)
  void clearSessionId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get rpcMethod => $_getSZ(5);
  @$pb.TagNumber(6)
  set rpcMethod($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRpcMethod() => $_has(5);
  @$pb.TagNumber(6)
  void clearRpcMethod() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get message => $_getSZ(6);
  @$pb.TagNumber(7)
  set message($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasMessage() => $_has(6);
  @$pb.TagNumber(7)
  void clearMessage() => $_clearField(7);

  @$pb.TagNumber(8)
  $pb.PbMap<$core.String, $core.String> get fields => $_getMap(7);

  @$pb.TagNumber(9)
  $core.String get caller => $_getSZ(8);
  @$pb.TagNumber(9)
  set caller($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasCaller() => $_has(8);
  @$pb.TagNumber(9)
  void clearCaller() => $_clearField(9);

  /// Relay path: ordered hops from the originator up through each
  /// relay before arriving on the stream being read. Empty when the
  /// entry was emitted by the holon whose stream the reader is
  /// consuming. See OBSERVABILITY.md §Organism Relay.
  @$pb.TagNumber(10)
  $pb.PbList<ChainHop> get chain => $_getList(9);
}

class ChainHop extends $pb.GeneratedMessage {
  factory ChainHop({
    $core.String? slug,
    $core.String? instanceUid,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    if (instanceUid != null) result.instanceUid = instanceUid;
    return result;
  }

  ChainHop._();

  factory ChainHop.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChainHop.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChainHop',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..aOS(2, _omitFieldNames ? '' : 'instanceUid')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChainHop clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChainHop copyWith(void Function(ChainHop) updates) =>
      super.copyWith((message) => updates(message as ChainHop)) as ChainHop;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChainHop create() => ChainHop._();
  @$core.override
  ChainHop createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChainHop getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ChainHop>(create);
  static ChainHop? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get instanceUid => $_getSZ(1);
  @$pb.TagNumber(2)
  set instanceUid($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInstanceUid() => $_has(1);
  @$pb.TagNumber(2)
  void clearInstanceUid() => $_clearField(2);
}

class MetricsRequest extends $pb.GeneratedMessage {
  factory MetricsRequest({
    $core.Iterable<$core.String>? namePrefixes,
    $core.bool? includeSessionRollup,
  }) {
    final result = create();
    if (namePrefixes != null) result.namePrefixes.addAll(namePrefixes);
    if (includeSessionRollup != null)
      result.includeSessionRollup = includeSessionRollup;
    return result;
  }

  MetricsRequest._();

  factory MetricsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MetricsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MetricsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'namePrefixes')
    ..aOB(2, _omitFieldNames ? '' : 'includeSessionRollup')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MetricsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MetricsRequest copyWith(void Function(MetricsRequest) updates) =>
      super.copyWith((message) => updates(message as MetricsRequest))
          as MetricsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MetricsRequest create() => MetricsRequest._();
  @$core.override
  MetricsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MetricsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MetricsRequest>(create);
  static MetricsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get namePrefixes => $_getList(0);

  @$pb.TagNumber(2)
  $core.bool get includeSessionRollup => $_getBF(1);
  @$pb.TagNumber(2)
  set includeSessionRollup($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIncludeSessionRollup() => $_has(1);
  @$pb.TagNumber(2)
  void clearIncludeSessionRollup() => $_clearField(2);
}

class MetricsSnapshot extends $pb.GeneratedMessage {
  factory MetricsSnapshot({
    $2.Timestamp? capturedAt,
    $core.String? slug,
    $core.String? instanceUid,
    $core.Iterable<MetricSample>? samples,
    $3.SessionMetrics? sessionRollup,
  }) {
    final result = create();
    if (capturedAt != null) result.capturedAt = capturedAt;
    if (slug != null) result.slug = slug;
    if (instanceUid != null) result.instanceUid = instanceUid;
    if (samples != null) result.samples.addAll(samples);
    if (sessionRollup != null) result.sessionRollup = sessionRollup;
    return result;
  }

  MetricsSnapshot._();

  factory MetricsSnapshot.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MetricsSnapshot.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MetricsSnapshot',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOM<$2.Timestamp>(1, _omitFieldNames ? '' : 'capturedAt',
        subBuilder: $2.Timestamp.create)
    ..aOS(2, _omitFieldNames ? '' : 'slug')
    ..aOS(3, _omitFieldNames ? '' : 'instanceUid')
    ..pPM<MetricSample>(4, _omitFieldNames ? '' : 'samples',
        subBuilder: MetricSample.create)
    ..aOM<$3.SessionMetrics>(5, _omitFieldNames ? '' : 'sessionRollup',
        subBuilder: $3.SessionMetrics.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MetricsSnapshot clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MetricsSnapshot copyWith(void Function(MetricsSnapshot) updates) =>
      super.copyWith((message) => updates(message as MetricsSnapshot))
          as MetricsSnapshot;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MetricsSnapshot create() => MetricsSnapshot._();
  @$core.override
  MetricsSnapshot createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MetricsSnapshot getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MetricsSnapshot>(create);
  static MetricsSnapshot? _defaultInstance;

  @$pb.TagNumber(1)
  $2.Timestamp get capturedAt => $_getN(0);
  @$pb.TagNumber(1)
  set capturedAt($2.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCapturedAt() => $_has(0);
  @$pb.TagNumber(1)
  void clearCapturedAt() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.Timestamp ensureCapturedAt() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get slug => $_getSZ(1);
  @$pb.TagNumber(2)
  set slug($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSlug() => $_has(1);
  @$pb.TagNumber(2)
  void clearSlug() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get instanceUid => $_getSZ(2);
  @$pb.TagNumber(3)
  set instanceUid($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasInstanceUid() => $_has(2);
  @$pb.TagNumber(3)
  void clearInstanceUid() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<MetricSample> get samples => $_getList(3);

  /// Reserved for v2; always empty in v1.
  @$pb.TagNumber(5)
  $3.SessionMetrics get sessionRollup => $_getN(4);
  @$pb.TagNumber(5)
  set sessionRollup($3.SessionMetrics value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasSessionRollup() => $_has(4);
  @$pb.TagNumber(5)
  void clearSessionRollup() => $_clearField(5);
  @$pb.TagNumber(5)
  $3.SessionMetrics ensureSessionRollup() => $_ensure(4);
}

enum MetricSample_Value { counter, gauge, histogram, notSet }

class MetricSample extends $pb.GeneratedMessage {
  factory MetricSample({
    $core.String? name,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? labels,
    $fixnum.Int64? counter,
    $core.double? gauge,
    HistogramSample? histogram,
    $core.String? help,
    $core.Iterable<ChainHop>? chain,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (labels != null) result.labels.addEntries(labels);
    if (counter != null) result.counter = counter;
    if (gauge != null) result.gauge = gauge;
    if (histogram != null) result.histogram = histogram;
    if (help != null) result.help = help;
    if (chain != null) result.chain.addAll(chain);
    return result;
  }

  MetricSample._();

  factory MetricSample.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MetricSample.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, MetricSample_Value>
      _MetricSample_ValueByTag = {
    3: MetricSample_Value.counter,
    4: MetricSample_Value.gauge,
    5: MetricSample_Value.histogram,
    0: MetricSample_Value.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MetricSample',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..oo(0, [3, 4, 5])
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..m<$core.String, $core.String>(2, _omitFieldNames ? '' : 'labels',
        entryClassName: 'MetricSample.LabelsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('holons.v1'))
    ..aInt64(3, _omitFieldNames ? '' : 'counter')
    ..aD(4, _omitFieldNames ? '' : 'gauge')
    ..aOM<HistogramSample>(5, _omitFieldNames ? '' : 'histogram',
        subBuilder: HistogramSample.create)
    ..aOS(6, _omitFieldNames ? '' : 'help')
    ..pPM<ChainHop>(7, _omitFieldNames ? '' : 'chain',
        subBuilder: ChainHop.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MetricSample clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MetricSample copyWith(void Function(MetricSample) updates) =>
      super.copyWith((message) => updates(message as MetricSample))
          as MetricSample;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MetricSample create() => MetricSample._();
  @$core.override
  MetricSample createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MetricSample getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MetricSample>(create);
  static MetricSample? _defaultInstance;

  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  MetricSample_Value whichValue() => _MetricSample_ValueByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  void clearValue() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbMap<$core.String, $core.String> get labels => $_getMap(1);

  @$pb.TagNumber(3)
  $fixnum.Int64 get counter => $_getI64(2);
  @$pb.TagNumber(3)
  set counter($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCounter() => $_has(2);
  @$pb.TagNumber(3)
  void clearCounter() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.double get gauge => $_getN(3);
  @$pb.TagNumber(4)
  set gauge($core.double value) => $_setDouble(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGauge() => $_has(3);
  @$pb.TagNumber(4)
  void clearGauge() => $_clearField(4);

  @$pb.TagNumber(5)
  HistogramSample get histogram => $_getN(4);
  @$pb.TagNumber(5)
  set histogram(HistogramSample value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasHistogram() => $_has(4);
  @$pb.TagNumber(5)
  void clearHistogram() => $_clearField(5);
  @$pb.TagNumber(5)
  HistogramSample ensureHistogram() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get help => $_getSZ(5);
  @$pb.TagNumber(6)
  set help($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasHelp() => $_has(5);
  @$pb.TagNumber(6)
  void clearHelp() => $_clearField(6);

  /// Relay path (see LogEntry.chain). Typically empty for metrics:
  /// the caller asks `child.Metrics()` directly on each direct child,
  /// so the stream identifies the source. Populated only when a holon
  /// folds a direct child's cached samples into its own snapshot.
  @$pb.TagNumber(7)
  $pb.PbList<ChainHop> get chain => $_getList(6);
}

class HistogramSample extends $pb.GeneratedMessage {
  factory HistogramSample({
    $core.Iterable<Bucket>? buckets,
    $fixnum.Int64? count,
    $core.double? sum,
  }) {
    final result = create();
    if (buckets != null) result.buckets.addAll(buckets);
    if (count != null) result.count = count;
    if (sum != null) result.sum = sum;
    return result;
  }

  HistogramSample._();

  factory HistogramSample.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HistogramSample.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HistogramSample',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPM<Bucket>(1, _omitFieldNames ? '' : 'buckets',
        subBuilder: Bucket.create)
    ..aInt64(2, _omitFieldNames ? '' : 'count')
    ..aD(3, _omitFieldNames ? '' : 'sum')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HistogramSample clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HistogramSample copyWith(void Function(HistogramSample) updates) =>
      super.copyWith((message) => updates(message as HistogramSample))
          as HistogramSample;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HistogramSample create() => HistogramSample._();
  @$core.override
  HistogramSample createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HistogramSample getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HistogramSample>(create);
  static HistogramSample? _defaultInstance;

  /// Cumulative buckets — bucket.count includes all samples
  /// where value <= bucket.upper_bound. Prometheus semantics.
  @$pb.TagNumber(1)
  $pb.PbList<Bucket> get buckets => $_getList(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get count => $_getI64(1);
  @$pb.TagNumber(2)
  set count($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearCount() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.double get sum => $_getN(2);
  @$pb.TagNumber(3)
  set sum($core.double value) => $_setDouble(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSum() => $_has(2);
  @$pb.TagNumber(3)
  void clearSum() => $_clearField(3);
}

class Bucket extends $pb.GeneratedMessage {
  factory Bucket({
    $core.double? upperBound,
    $fixnum.Int64? count,
  }) {
    final result = create();
    if (upperBound != null) result.upperBound = upperBound;
    if (count != null) result.count = count;
    return result;
  }

  Bucket._();

  factory Bucket.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Bucket.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Bucket',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aD(1, _omitFieldNames ? '' : 'upperBound')
    ..aInt64(2, _omitFieldNames ? '' : 'count')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Bucket clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Bucket copyWith(void Function(Bucket) updates) =>
      super.copyWith((message) => updates(message as Bucket)) as Bucket;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Bucket create() => Bucket._();
  @$core.override
  Bucket createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Bucket getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Bucket>(create);
  static Bucket? _defaultInstance;

  @$pb.TagNumber(1)
  $core.double get upperBound => $_getN(0);
  @$pb.TagNumber(1)
  set upperBound($core.double value) => $_setDouble(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUpperBound() => $_has(0);
  @$pb.TagNumber(1)
  void clearUpperBound() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get count => $_getI64(1);
  @$pb.TagNumber(2)
  set count($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearCount() => $_clearField(2);
}

class EventsRequest extends $pb.GeneratedMessage {
  factory EventsRequest({
    $core.Iterable<EventType>? types,
    $1.Duration? since,
    $core.bool? follow,
  }) {
    final result = create();
    if (types != null) result.types.addAll(types);
    if (since != null) result.since = since;
    if (follow != null) result.follow = follow;
    return result;
  }

  EventsRequest._();

  factory EventsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EventsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EventsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pc<EventType>(1, _omitFieldNames ? '' : 'types', $pb.PbFieldType.KE,
        valueOf: EventType.valueOf,
        enumValues: EventType.values,
        defaultEnumValue: EventType.EVENT_TYPE_UNSPECIFIED)
    ..aOM<$1.Duration>(2, _omitFieldNames ? '' : 'since',
        subBuilder: $1.Duration.create)
    ..aOB(3, _omitFieldNames ? '' : 'follow')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventsRequest copyWith(void Function(EventsRequest) updates) =>
      super.copyWith((message) => updates(message as EventsRequest))
          as EventsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EventsRequest create() => EventsRequest._();
  @$core.override
  EventsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EventsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EventsRequest>(create);
  static EventsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<EventType> get types => $_getList(0);

  @$pb.TagNumber(2)
  $1.Duration get since => $_getN(1);
  @$pb.TagNumber(2)
  set since($1.Duration value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSince() => $_has(1);
  @$pb.TagNumber(2)
  void clearSince() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Duration ensureSince() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.bool get follow => $_getBF(2);
  @$pb.TagNumber(3)
  set follow($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFollow() => $_has(2);
  @$pb.TagNumber(3)
  void clearFollow() => $_clearField(3);
}

class EventInfo extends $pb.GeneratedMessage {
  factory EventInfo({
    $2.Timestamp? ts,
    EventType? type,
    $core.String? slug,
    $core.String? instanceUid,
    $core.String? sessionId,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? payload,
    $core.Iterable<ChainHop>? chain,
  }) {
    final result = create();
    if (ts != null) result.ts = ts;
    if (type != null) result.type = type;
    if (slug != null) result.slug = slug;
    if (instanceUid != null) result.instanceUid = instanceUid;
    if (sessionId != null) result.sessionId = sessionId;
    if (payload != null) result.payload.addEntries(payload);
    if (chain != null) result.chain.addAll(chain);
    return result;
  }

  EventInfo._();

  factory EventInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EventInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EventInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOM<$2.Timestamp>(1, _omitFieldNames ? '' : 'ts',
        subBuilder: $2.Timestamp.create)
    ..aE<EventType>(2, _omitFieldNames ? '' : 'type',
        enumValues: EventType.values)
    ..aOS(3, _omitFieldNames ? '' : 'slug')
    ..aOS(4, _omitFieldNames ? '' : 'instanceUid')
    ..aOS(5, _omitFieldNames ? '' : 'sessionId')
    ..m<$core.String, $core.String>(6, _omitFieldNames ? '' : 'payload',
        entryClassName: 'EventInfo.PayloadEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('holons.v1'))
    ..pPM<ChainHop>(7, _omitFieldNames ? '' : 'chain',
        subBuilder: ChainHop.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventInfo copyWith(void Function(EventInfo) updates) =>
      super.copyWith((message) => updates(message as EventInfo)) as EventInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EventInfo create() => EventInfo._();
  @$core.override
  EventInfo createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EventInfo getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<EventInfo>(create);
  static EventInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $2.Timestamp get ts => $_getN(0);
  @$pb.TagNumber(1)
  set ts($2.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTs() => $_has(0);
  @$pb.TagNumber(1)
  void clearTs() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.Timestamp ensureTs() => $_ensure(0);

  @$pb.TagNumber(2)
  EventType get type => $_getN(1);
  @$pb.TagNumber(2)
  set type(EventType value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get slug => $_getSZ(2);
  @$pb.TagNumber(3)
  set slug($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSlug() => $_has(2);
  @$pb.TagNumber(3)
  void clearSlug() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get instanceUid => $_getSZ(3);
  @$pb.TagNumber(4)
  set instanceUid($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasInstanceUid() => $_has(3);
  @$pb.TagNumber(4)
  void clearInstanceUid() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get sessionId => $_getSZ(4);
  @$pb.TagNumber(5)
  set sessionId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSessionId() => $_has(4);
  @$pb.TagNumber(5)
  void clearSessionId() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbMap<$core.String, $core.String> get payload => $_getMap(5);

  /// Relay path (see LogEntry.chain, same semantics).
  @$pb.TagNumber(7)
  $pb.PbList<ChainHop> get chain => $_getList(6);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
