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

import 'observability.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'observability.pbenum.dart';

class LogsRequest extends $pb.GeneratedMessage {
  factory LogsRequest({
    SeverityNumber? minSeverityNumber,
    $core.Iterable<$core.String>? sessionIds,
    $core.Iterable<$core.String>? rpcMethods,
    $1.Duration? since,
    $core.bool? follow,
  }) {
    final result = create();
    if (minSeverityNumber != null) result.minSeverityNumber = minSeverityNumber;
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
    ..aE<SeverityNumber>(1, _omitFieldNames ? '' : 'minSeverityNumber',
        enumValues: SeverityNumber.values)
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
  SeverityNumber get minSeverityNumber => $_getN(0);
  @$pb.TagNumber(1)
  set minSeverityNumber(SeverityNumber value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMinSeverityNumber() => $_has(0);
  @$pb.TagNumber(1)
  void clearMinSeverityNumber() => $_clearField(1);

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

class MetricsRequest extends $pb.GeneratedMessage {
  factory MetricsRequest({
    $core.Iterable<$core.String>? namePrefixes,
  }) {
    final result = create();
    if (namePrefixes != null) result.namePrefixes.addAll(namePrefixes);
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
}

class EventsRequest extends $pb.GeneratedMessage {
  factory EventsRequest({
    $core.Iterable<$core.String>? eventNames,
    $1.Duration? since,
    $core.bool? follow,
  }) {
    final result = create();
    if (eventNames != null) result.eventNames.addAll(eventNames);
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
    ..pPS(1, _omitFieldNames ? '' : 'eventNames')
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
  $pb.PbList<$core.String> get eventNames => $_getList(0);

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

enum AnyValue_Value { stringValue, boolValue, intValue, doubleValue, notSet }

/// Structurally mirrors opentelemetry.proto.common.v1.AnyValue.
class AnyValue extends $pb.GeneratedMessage {
  factory AnyValue({
    $core.String? stringValue,
    $core.bool? boolValue,
    $fixnum.Int64? intValue,
    $core.double? doubleValue,
  }) {
    final result = create();
    if (stringValue != null) result.stringValue = stringValue;
    if (boolValue != null) result.boolValue = boolValue;
    if (intValue != null) result.intValue = intValue;
    if (doubleValue != null) result.doubleValue = doubleValue;
    return result;
  }

  AnyValue._();

  factory AnyValue.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AnyValue.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, AnyValue_Value> _AnyValue_ValueByTag = {
    1: AnyValue_Value.stringValue,
    2: AnyValue_Value.boolValue,
    3: AnyValue_Value.intValue,
    4: AnyValue_Value.doubleValue,
    0: AnyValue_Value.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AnyValue',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..oo(0, [1, 2, 3, 4])
    ..aOS(1, _omitFieldNames ? '' : 'stringValue')
    ..aOB(2, _omitFieldNames ? '' : 'boolValue')
    ..aInt64(3, _omitFieldNames ? '' : 'intValue')
    ..aD(4, _omitFieldNames ? '' : 'doubleValue')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AnyValue clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AnyValue copyWith(void Function(AnyValue) updates) =>
      super.copyWith((message) => updates(message as AnyValue)) as AnyValue;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AnyValue create() => AnyValue._();
  @$core.override
  AnyValue createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AnyValue getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AnyValue>(create);
  static AnyValue? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  AnyValue_Value whichValue() => _AnyValue_ValueByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  void clearValue() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get stringValue => $_getSZ(0);
  @$pb.TagNumber(1)
  set stringValue($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStringValue() => $_has(0);
  @$pb.TagNumber(1)
  void clearStringValue() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get boolValue => $_getBF(1);
  @$pb.TagNumber(2)
  set boolValue($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBoolValue() => $_has(1);
  @$pb.TagNumber(2)
  void clearBoolValue() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get intValue => $_getI64(2);
  @$pb.TagNumber(3)
  set intValue($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIntValue() => $_has(2);
  @$pb.TagNumber(3)
  void clearIntValue() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.double get doubleValue => $_getN(3);
  @$pb.TagNumber(4)
  set doubleValue($core.double value) => $_setDouble(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDoubleValue() => $_has(3);
  @$pb.TagNumber(4)
  void clearDoubleValue() => $_clearField(4);
}

/// Structurally mirrors opentelemetry.proto.common.v1.KeyValue.
class KeyValue extends $pb.GeneratedMessage {
  factory KeyValue({
    $core.String? key,
    AnyValue? value,
  }) {
    final result = create();
    if (key != null) result.key = key;
    if (value != null) result.value = value;
    return result;
  }

  KeyValue._();

  factory KeyValue.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory KeyValue.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KeyValue',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'key')
    ..aOM<AnyValue>(2, _omitFieldNames ? '' : 'value',
        subBuilder: AnyValue.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KeyValue clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KeyValue copyWith(void Function(KeyValue) updates) =>
      super.copyWith((message) => updates(message as KeyValue)) as KeyValue;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static KeyValue create() => KeyValue._();
  @$core.override
  KeyValue createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static KeyValue getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<KeyValue>(create);
  static KeyValue? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get key => $_getSZ(0);
  @$pb.TagNumber(1)
  set key($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearKey() => $_clearField(1);

  @$pb.TagNumber(2)
  AnyValue get value => $_getN(1);
  @$pb.TagNumber(2)
  set value(AnyValue value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasValue() => $_has(1);
  @$pb.TagNumber(2)
  void clearValue() => $_clearField(2);
  @$pb.TagNumber(2)
  AnyValue ensureValue() => $_ensure(1);
}

/// Structurally mirrors opentelemetry.proto.resource.v1.Resource.
class Resource extends $pb.GeneratedMessage {
  factory Resource({
    $core.Iterable<KeyValue>? attributes,
    $core.int? droppedAttributesCount,
  }) {
    final result = create();
    if (attributes != null) result.attributes.addAll(attributes);
    if (droppedAttributesCount != null)
      result.droppedAttributesCount = droppedAttributesCount;
    return result;
  }

  Resource._();

  factory Resource.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Resource.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Resource',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPM<KeyValue>(1, _omitFieldNames ? '' : 'attributes',
        subBuilder: KeyValue.create)
    ..aI(2, _omitFieldNames ? '' : 'droppedAttributesCount',
        fieldType: $pb.PbFieldType.OU3)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Resource clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Resource copyWith(void Function(Resource) updates) =>
      super.copyWith((message) => updates(message as Resource)) as Resource;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Resource create() => Resource._();
  @$core.override
  Resource createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Resource getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Resource>(create);
  static Resource? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<KeyValue> get attributes => $_getList(0);

  @$pb.TagNumber(2)
  $core.int get droppedAttributesCount => $_getIZ(1);
  @$pb.TagNumber(2)
  set droppedAttributesCount($core.int value) => $_setUnsignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDroppedAttributesCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearDroppedAttributesCount() => $_clearField(2);
}

/// Structurally mirrors opentelemetry.proto.logs.v1.LogRecord.
class LogRecord extends $pb.GeneratedMessage {
  factory LogRecord({
    $fixnum.Int64? timeUnixNano,
    SeverityNumber? severityNumber,
    $core.String? severityText,
    AnyValue? body,
    $core.Iterable<KeyValue>? attributes,
    $core.int? droppedAttributesCount,
    $core.int? flags,
    $core.List<$core.int>? traceId,
    $core.List<$core.int>? spanId,
    $fixnum.Int64? observedTimeUnixNano,
    $core.String? eventName,
    $core.Iterable<$core.String>? chain,
  }) {
    final result = create();
    if (timeUnixNano != null) result.timeUnixNano = timeUnixNano;
    if (severityNumber != null) result.severityNumber = severityNumber;
    if (severityText != null) result.severityText = severityText;
    if (body != null) result.body = body;
    if (attributes != null) result.attributes.addAll(attributes);
    if (droppedAttributesCount != null)
      result.droppedAttributesCount = droppedAttributesCount;
    if (flags != null) result.flags = flags;
    if (traceId != null) result.traceId = traceId;
    if (spanId != null) result.spanId = spanId;
    if (observedTimeUnixNano != null)
      result.observedTimeUnixNano = observedTimeUnixNano;
    if (eventName != null) result.eventName = eventName;
    if (chain != null) result.chain.addAll(chain);
    return result;
  }

  LogRecord._();

  factory LogRecord.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LogRecord.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LogRecord',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'timeUnixNano', $pb.PbFieldType.OF6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<SeverityNumber>(2, _omitFieldNames ? '' : 'severityNumber',
        enumValues: SeverityNumber.values)
    ..aOS(3, _omitFieldNames ? '' : 'severityText')
    ..aOM<AnyValue>(5, _omitFieldNames ? '' : 'body',
        subBuilder: AnyValue.create)
    ..pPM<KeyValue>(6, _omitFieldNames ? '' : 'attributes',
        subBuilder: KeyValue.create)
    ..aI(7, _omitFieldNames ? '' : 'droppedAttributesCount',
        fieldType: $pb.PbFieldType.OU3)
    ..aI(8, _omitFieldNames ? '' : 'flags', fieldType: $pb.PbFieldType.OU3)
    ..a<$core.List<$core.int>>(
        9, _omitFieldNames ? '' : 'traceId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        10, _omitFieldNames ? '' : 'spanId', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        11, _omitFieldNames ? '' : 'observedTimeUnixNano', $pb.PbFieldType.OF6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(20, _omitFieldNames ? '' : 'eventName')
    ..pPS(21, _omitFieldNames ? '' : 'chain')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogRecord clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogRecord copyWith(void Function(LogRecord) updates) =>
      super.copyWith((message) => updates(message as LogRecord)) as LogRecord;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LogRecord create() => LogRecord._();
  @$core.override
  LogRecord createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LogRecord getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<LogRecord>(create);
  static LogRecord? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get timeUnixNano => $_getI64(0);
  @$pb.TagNumber(1)
  set timeUnixNano($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTimeUnixNano() => $_has(0);
  @$pb.TagNumber(1)
  void clearTimeUnixNano() => $_clearField(1);

  @$pb.TagNumber(2)
  SeverityNumber get severityNumber => $_getN(1);
  @$pb.TagNumber(2)
  set severityNumber(SeverityNumber value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSeverityNumber() => $_has(1);
  @$pb.TagNumber(2)
  void clearSeverityNumber() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get severityText => $_getSZ(2);
  @$pb.TagNumber(3)
  set severityText($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSeverityText() => $_has(2);
  @$pb.TagNumber(3)
  void clearSeverityText() => $_clearField(3);

  @$pb.TagNumber(5)
  AnyValue get body => $_getN(3);
  @$pb.TagNumber(5)
  set body(AnyValue value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasBody() => $_has(3);
  @$pb.TagNumber(5)
  void clearBody() => $_clearField(5);
  @$pb.TagNumber(5)
  AnyValue ensureBody() => $_ensure(3);

  @$pb.TagNumber(6)
  $pb.PbList<KeyValue> get attributes => $_getList(4);

  @$pb.TagNumber(7)
  $core.int get droppedAttributesCount => $_getIZ(5);
  @$pb.TagNumber(7)
  set droppedAttributesCount($core.int value) => $_setUnsignedInt32(5, value);
  @$pb.TagNumber(7)
  $core.bool hasDroppedAttributesCount() => $_has(5);
  @$pb.TagNumber(7)
  void clearDroppedAttributesCount() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.int get flags => $_getIZ(6);
  @$pb.TagNumber(8)
  set flags($core.int value) => $_setUnsignedInt32(6, value);
  @$pb.TagNumber(8)
  $core.bool hasFlags() => $_has(6);
  @$pb.TagNumber(8)
  void clearFlags() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.List<$core.int> get traceId => $_getN(7);
  @$pb.TagNumber(9)
  set traceId($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(9)
  $core.bool hasTraceId() => $_has(7);
  @$pb.TagNumber(9)
  void clearTraceId() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.List<$core.int> get spanId => $_getN(8);
  @$pb.TagNumber(10)
  set spanId($core.List<$core.int> value) => $_setBytes(8, value);
  @$pb.TagNumber(10)
  $core.bool hasSpanId() => $_has(8);
  @$pb.TagNumber(10)
  void clearSpanId() => $_clearField(10);

  @$pb.TagNumber(11)
  $fixnum.Int64 get observedTimeUnixNano => $_getI64(9);
  @$pb.TagNumber(11)
  set observedTimeUnixNano($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(11)
  $core.bool hasObservedTimeUnixNano() => $_has(9);
  @$pb.TagNumber(11)
  void clearObservedTimeUnixNano() => $_clearField(11);

  @$pb.TagNumber(20)
  $core.String get eventName => $_getSZ(10);
  @$pb.TagNumber(20)
  set eventName($core.String value) => $_setString(10, value);
  @$pb.TagNumber(20)
  $core.bool hasEventName() => $_has(10);
  @$pb.TagNumber(20)
  void clearEventName() => $_clearField(20);

  @$pb.TagNumber(21)
  $pb.PbList<$core.String> get chain => $_getList(11);
}

enum Metric_Data { gauge, sum, histogram, notSet }

/// Structurally mirrors opentelemetry.proto.metrics.v1.Metric.
class Metric extends $pb.GeneratedMessage {
  factory Metric({
    $core.String? name,
    $core.String? description,
    $core.String? unit,
    Gauge? gauge,
    Sum? sum,
    Histogram? histogram,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (unit != null) result.unit = unit;
    if (gauge != null) result.gauge = gauge;
    if (sum != null) result.sum = sum;
    if (histogram != null) result.histogram = histogram;
    return result;
  }

  Metric._();

  factory Metric.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Metric.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, Metric_Data> _Metric_DataByTag = {
    5: Metric_Data.gauge,
    7: Metric_Data.sum,
    9: Metric_Data.histogram,
    0: Metric_Data.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Metric',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..oo(0, [5, 7, 9])
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOS(3, _omitFieldNames ? '' : 'unit')
    ..aOM<Gauge>(5, _omitFieldNames ? '' : 'gauge', subBuilder: Gauge.create)
    ..aOM<Sum>(7, _omitFieldNames ? '' : 'sum', subBuilder: Sum.create)
    ..aOM<Histogram>(9, _omitFieldNames ? '' : 'histogram',
        subBuilder: Histogram.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Metric clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Metric copyWith(void Function(Metric) updates) =>
      super.copyWith((message) => updates(message as Metric)) as Metric;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Metric create() => Metric._();
  @$core.override
  Metric createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Metric getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Metric>(create);
  static Metric? _defaultInstance;

  @$pb.TagNumber(5)
  @$pb.TagNumber(7)
  @$pb.TagNumber(9)
  Metric_Data whichData() => _Metric_DataByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(5)
  @$pb.TagNumber(7)
  @$pb.TagNumber(9)
  void clearData() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get unit => $_getSZ(2);
  @$pb.TagNumber(3)
  set unit($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasUnit() => $_has(2);
  @$pb.TagNumber(3)
  void clearUnit() => $_clearField(3);

  @$pb.TagNumber(5)
  Gauge get gauge => $_getN(3);
  @$pb.TagNumber(5)
  set gauge(Gauge value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasGauge() => $_has(3);
  @$pb.TagNumber(5)
  void clearGauge() => $_clearField(5);
  @$pb.TagNumber(5)
  Gauge ensureGauge() => $_ensure(3);

  @$pb.TagNumber(7)
  Sum get sum => $_getN(4);
  @$pb.TagNumber(7)
  set sum(Sum value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasSum() => $_has(4);
  @$pb.TagNumber(7)
  void clearSum() => $_clearField(7);
  @$pb.TagNumber(7)
  Sum ensureSum() => $_ensure(4);

  @$pb.TagNumber(9)
  Histogram get histogram => $_getN(5);
  @$pb.TagNumber(9)
  set histogram(Histogram value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasHistogram() => $_has(5);
  @$pb.TagNumber(9)
  void clearHistogram() => $_clearField(9);
  @$pb.TagNumber(9)
  Histogram ensureHistogram() => $_ensure(5);
}

class Gauge extends $pb.GeneratedMessage {
  factory Gauge({
    $core.Iterable<NumberDataPoint>? dataPoints,
  }) {
    final result = create();
    if (dataPoints != null) result.dataPoints.addAll(dataPoints);
    return result;
  }

  Gauge._();

  factory Gauge.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Gauge.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Gauge',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPM<NumberDataPoint>(1, _omitFieldNames ? '' : 'dataPoints',
        subBuilder: NumberDataPoint.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Gauge clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Gauge copyWith(void Function(Gauge) updates) =>
      super.copyWith((message) => updates(message as Gauge)) as Gauge;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Gauge create() => Gauge._();
  @$core.override
  Gauge createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Gauge getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Gauge>(create);
  static Gauge? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<NumberDataPoint> get dataPoints => $_getList(0);
}

class Sum extends $pb.GeneratedMessage {
  factory Sum({
    $core.Iterable<NumberDataPoint>? dataPoints,
    AggregationTemporality? aggregationTemporality,
    $core.bool? isMonotonic,
  }) {
    final result = create();
    if (dataPoints != null) result.dataPoints.addAll(dataPoints);
    if (aggregationTemporality != null)
      result.aggregationTemporality = aggregationTemporality;
    if (isMonotonic != null) result.isMonotonic = isMonotonic;
    return result;
  }

  Sum._();

  factory Sum.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Sum.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Sum',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPM<NumberDataPoint>(1, _omitFieldNames ? '' : 'dataPoints',
        subBuilder: NumberDataPoint.create)
    ..aE<AggregationTemporality>(
        2, _omitFieldNames ? '' : 'aggregationTemporality',
        enumValues: AggregationTemporality.values)
    ..aOB(3, _omitFieldNames ? '' : 'isMonotonic')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Sum clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Sum copyWith(void Function(Sum) updates) =>
      super.copyWith((message) => updates(message as Sum)) as Sum;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Sum create() => Sum._();
  @$core.override
  Sum createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Sum getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Sum>(create);
  static Sum? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<NumberDataPoint> get dataPoints => $_getList(0);

  @$pb.TagNumber(2)
  AggregationTemporality get aggregationTemporality => $_getN(1);
  @$pb.TagNumber(2)
  set aggregationTemporality(AggregationTemporality value) =>
      $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAggregationTemporality() => $_has(1);
  @$pb.TagNumber(2)
  void clearAggregationTemporality() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isMonotonic => $_getBF(2);
  @$pb.TagNumber(3)
  set isMonotonic($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsMonotonic() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsMonotonic() => $_clearField(3);
}

class Histogram extends $pb.GeneratedMessage {
  factory Histogram({
    $core.Iterable<HistogramDataPoint>? dataPoints,
    AggregationTemporality? aggregationTemporality,
  }) {
    final result = create();
    if (dataPoints != null) result.dataPoints.addAll(dataPoints);
    if (aggregationTemporality != null)
      result.aggregationTemporality = aggregationTemporality;
    return result;
  }

  Histogram._();

  factory Histogram.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Histogram.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Histogram',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPM<HistogramDataPoint>(1, _omitFieldNames ? '' : 'dataPoints',
        subBuilder: HistogramDataPoint.create)
    ..aE<AggregationTemporality>(
        2, _omitFieldNames ? '' : 'aggregationTemporality',
        enumValues: AggregationTemporality.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Histogram clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Histogram copyWith(void Function(Histogram) updates) =>
      super.copyWith((message) => updates(message as Histogram)) as Histogram;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Histogram create() => Histogram._();
  @$core.override
  Histogram createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Histogram getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Histogram>(create);
  static Histogram? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<HistogramDataPoint> get dataPoints => $_getList(0);

  @$pb.TagNumber(2)
  AggregationTemporality get aggregationTemporality => $_getN(1);
  @$pb.TagNumber(2)
  set aggregationTemporality(AggregationTemporality value) =>
      $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAggregationTemporality() => $_has(1);
  @$pb.TagNumber(2)
  void clearAggregationTemporality() => $_clearField(2);
}

enum NumberDataPoint_Value { asDouble, asInt, notSet }

class NumberDataPoint extends $pb.GeneratedMessage {
  factory NumberDataPoint({
    $fixnum.Int64? startTimeUnixNano,
    $fixnum.Int64? timeUnixNano,
    $core.double? asDouble,
    $fixnum.Int64? asInt,
    $core.Iterable<KeyValue>? attributes,
  }) {
    final result = create();
    if (startTimeUnixNano != null) result.startTimeUnixNano = startTimeUnixNano;
    if (timeUnixNano != null) result.timeUnixNano = timeUnixNano;
    if (asDouble != null) result.asDouble = asDouble;
    if (asInt != null) result.asInt = asInt;
    if (attributes != null) result.attributes.addAll(attributes);
    return result;
  }

  NumberDataPoint._();

  factory NumberDataPoint.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory NumberDataPoint.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, NumberDataPoint_Value>
      _NumberDataPoint_ValueByTag = {
    4: NumberDataPoint_Value.asDouble,
    6: NumberDataPoint_Value.asInt,
    0: NumberDataPoint_Value.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'NumberDataPoint',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..oo(0, [4, 6])
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'startTimeUnixNano', $pb.PbFieldType.OF6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'timeUnixNano', $pb.PbFieldType.OF6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aD(4, _omitFieldNames ? '' : 'asDouble')
    ..aInt64(6, _omitFieldNames ? '' : 'asInt')
    ..pPM<KeyValue>(7, _omitFieldNames ? '' : 'attributes',
        subBuilder: KeyValue.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NumberDataPoint clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NumberDataPoint copyWith(void Function(NumberDataPoint) updates) =>
      super.copyWith((message) => updates(message as NumberDataPoint))
          as NumberDataPoint;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static NumberDataPoint create() => NumberDataPoint._();
  @$core.override
  NumberDataPoint createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static NumberDataPoint getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<NumberDataPoint>(create);
  static NumberDataPoint? _defaultInstance;

  @$pb.TagNumber(4)
  @$pb.TagNumber(6)
  NumberDataPoint_Value whichValue() =>
      _NumberDataPoint_ValueByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(4)
  @$pb.TagNumber(6)
  void clearValue() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(2)
  $fixnum.Int64 get startTimeUnixNano => $_getI64(0);
  @$pb.TagNumber(2)
  set startTimeUnixNano($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(2)
  $core.bool hasStartTimeUnixNano() => $_has(0);
  @$pb.TagNumber(2)
  void clearStartTimeUnixNano() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get timeUnixNano => $_getI64(1);
  @$pb.TagNumber(3)
  set timeUnixNano($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(3)
  $core.bool hasTimeUnixNano() => $_has(1);
  @$pb.TagNumber(3)
  void clearTimeUnixNano() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.double get asDouble => $_getN(2);
  @$pb.TagNumber(4)
  set asDouble($core.double value) => $_setDouble(2, value);
  @$pb.TagNumber(4)
  $core.bool hasAsDouble() => $_has(2);
  @$pb.TagNumber(4)
  void clearAsDouble() => $_clearField(4);

  @$pb.TagNumber(6)
  $fixnum.Int64 get asInt => $_getI64(3);
  @$pb.TagNumber(6)
  set asInt($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(6)
  $core.bool hasAsInt() => $_has(3);
  @$pb.TagNumber(6)
  void clearAsInt() => $_clearField(6);

  @$pb.TagNumber(7)
  $pb.PbList<KeyValue> get attributes => $_getList(4);
}

class HistogramDataPoint extends $pb.GeneratedMessage {
  factory HistogramDataPoint({
    $fixnum.Int64? startTimeUnixNano,
    $fixnum.Int64? timeUnixNano,
    $fixnum.Int64? count,
    $core.double? sum,
    $core.Iterable<$fixnum.Int64>? bucketCounts,
    $core.Iterable<$core.double>? explicitBounds,
    $core.Iterable<KeyValue>? attributes,
    $core.double? min,
    $core.double? max,
  }) {
    final result = create();
    if (startTimeUnixNano != null) result.startTimeUnixNano = startTimeUnixNano;
    if (timeUnixNano != null) result.timeUnixNano = timeUnixNano;
    if (count != null) result.count = count;
    if (sum != null) result.sum = sum;
    if (bucketCounts != null) result.bucketCounts.addAll(bucketCounts);
    if (explicitBounds != null) result.explicitBounds.addAll(explicitBounds);
    if (attributes != null) result.attributes.addAll(attributes);
    if (min != null) result.min = min;
    if (max != null) result.max = max;
    return result;
  }

  HistogramDataPoint._();

  factory HistogramDataPoint.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HistogramDataPoint.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HistogramDataPoint',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'startTimeUnixNano', $pb.PbFieldType.OF6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'timeUnixNano', $pb.PbFieldType.OF6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(4, _omitFieldNames ? '' : 'count', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aD(5, _omitFieldNames ? '' : 'sum')
    ..p<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'bucketCounts', $pb.PbFieldType.KU6)
    ..p<$core.double>(
        7, _omitFieldNames ? '' : 'explicitBounds', $pb.PbFieldType.KD)
    ..pPM<KeyValue>(9, _omitFieldNames ? '' : 'attributes',
        subBuilder: KeyValue.create)
    ..aD(11, _omitFieldNames ? '' : 'min')
    ..aD(12, _omitFieldNames ? '' : 'max')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HistogramDataPoint clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HistogramDataPoint copyWith(void Function(HistogramDataPoint) updates) =>
      super.copyWith((message) => updates(message as HistogramDataPoint))
          as HistogramDataPoint;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HistogramDataPoint create() => HistogramDataPoint._();
  @$core.override
  HistogramDataPoint createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HistogramDataPoint getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HistogramDataPoint>(create);
  static HistogramDataPoint? _defaultInstance;

  @$pb.TagNumber(2)
  $fixnum.Int64 get startTimeUnixNano => $_getI64(0);
  @$pb.TagNumber(2)
  set startTimeUnixNano($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(2)
  $core.bool hasStartTimeUnixNano() => $_has(0);
  @$pb.TagNumber(2)
  void clearStartTimeUnixNano() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get timeUnixNano => $_getI64(1);
  @$pb.TagNumber(3)
  set timeUnixNano($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(3)
  $core.bool hasTimeUnixNano() => $_has(1);
  @$pb.TagNumber(3)
  void clearTimeUnixNano() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get count => $_getI64(2);
  @$pb.TagNumber(4)
  set count($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(4)
  $core.bool hasCount() => $_has(2);
  @$pb.TagNumber(4)
  void clearCount() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.double get sum => $_getN(3);
  @$pb.TagNumber(5)
  set sum($core.double value) => $_setDouble(3, value);
  @$pb.TagNumber(5)
  $core.bool hasSum() => $_has(3);
  @$pb.TagNumber(5)
  void clearSum() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<$fixnum.Int64> get bucketCounts => $_getList(4);

  @$pb.TagNumber(7)
  $pb.PbList<$core.double> get explicitBounds => $_getList(5);

  @$pb.TagNumber(9)
  $pb.PbList<KeyValue> get attributes => $_getList(6);

  @$pb.TagNumber(11)
  $core.double get min => $_getN(7);
  @$pb.TagNumber(11)
  set min($core.double value) => $_setDouble(7, value);
  @$pb.TagNumber(11)
  $core.bool hasMin() => $_has(7);
  @$pb.TagNumber(11)
  void clearMin() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.double get max => $_getN(8);
  @$pb.TagNumber(12)
  set max($core.double value) => $_setDouble(8, value);
  @$pb.TagNumber(12)
  $core.bool hasMax() => $_has(8);
  @$pb.TagNumber(12)
  void clearMax() => $_clearField(12);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
