// This is a generated file - do not edit.
//
// Generated from observability_cascade/v1/service.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class RunRequest extends $pb.GeneratedMessage {
  factory RunRequest() => create();

  RunRequest._();

  factory RunRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RunRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RunRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'observability_cascade.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunRequest copyWith(void Function(RunRequest) updates) =>
      super.copyWith((message) => updates(message as RunRequest)) as RunRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RunRequest create() => RunRequest._();
  @$core.override
  RunRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RunRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RunRequest>(create);
  static RunRequest? _defaultInstance;
}

class PhaseResult extends $pb.GeneratedMessage {
  factory PhaseResult({
    $core.String? name,
    $core.int? pass,
    $core.int? fail,
    $core.Iterable<$core.String>? failures,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (pass != null) result.pass = pass;
    if (fail != null) result.fail = fail;
    if (failures != null) result.failures.addAll(failures);
    return result;
  }

  PhaseResult._();

  factory PhaseResult.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PhaseResult.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PhaseResult',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'observability_cascade.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aI(2, _omitFieldNames ? '' : 'pass')
    ..aI(3, _omitFieldNames ? '' : 'fail')
    ..pPS(4, _omitFieldNames ? '' : 'failures')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PhaseResult clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PhaseResult copyWith(void Function(PhaseResult) updates) =>
      super.copyWith((message) => updates(message as PhaseResult))
          as PhaseResult;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PhaseResult create() => PhaseResult._();
  @$core.override
  PhaseResult createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PhaseResult getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PhaseResult>(create);
  static PhaseResult? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get pass => $_getIZ(1);
  @$pb.TagNumber(2)
  set pass($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPass() => $_has(1);
  @$pb.TagNumber(2)
  void clearPass() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get fail => $_getIZ(2);
  @$pb.TagNumber(3)
  set fail($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFail() => $_has(2);
  @$pb.TagNumber(3)
  void clearFail() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get failures => $_getList(3);
}

class CascadeReport extends $pb.GeneratedMessage {
  factory CascadeReport({
    $core.int? ticks,
    $core.int? pass,
    $core.int? fail,
    $core.Iterable<PhaseResult>? phases,
    $core.String? name,
  }) {
    final result = create();
    if (ticks != null) result.ticks = ticks;
    if (pass != null) result.pass = pass;
    if (fail != null) result.fail = fail;
    if (phases != null) result.phases.addAll(phases);
    if (name != null) result.name = name;
    return result;
  }

  CascadeReport._();

  factory CascadeReport.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CascadeReport.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CascadeReport',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'observability_cascade.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'ticks')
    ..aI(2, _omitFieldNames ? '' : 'pass')
    ..aI(3, _omitFieldNames ? '' : 'fail')
    ..pPM<PhaseResult>(4, _omitFieldNames ? '' : 'phases',
        subBuilder: PhaseResult.create)
    ..aOS(5, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CascadeReport clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CascadeReport copyWith(void Function(CascadeReport) updates) =>
      super.copyWith((message) => updates(message as CascadeReport))
          as CascadeReport;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CascadeReport create() => CascadeReport._();
  @$core.override
  CascadeReport createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CascadeReport getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CascadeReport>(create);
  static CascadeReport? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get ticks => $_getIZ(0);
  @$pb.TagNumber(1)
  set ticks($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTicks() => $_has(0);
  @$pb.TagNumber(1)
  void clearTicks() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get pass => $_getIZ(1);
  @$pb.TagNumber(2)
  set pass($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPass() => $_has(1);
  @$pb.TagNumber(2)
  void clearPass() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get fail => $_getIZ(2);
  @$pb.TagNumber(3)
  set fail($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFail() => $_has(2);
  @$pb.TagNumber(3)
  void clearFail() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<PhaseResult> get phases => $_getList(3);

  @$pb.TagNumber(5)
  $core.String get name => $_getSZ(4);
  @$pb.TagNumber(5)
  set name($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasName() => $_has(4);
  @$pb.TagNumber(5)
  void clearName() => $_clearField(5);
}

class MultiPatternReport extends $pb.GeneratedMessage {
  factory MultiPatternReport({
    $core.Iterable<CascadeReport>? patterns,
    $core.int? totalPass,
    $core.int? totalFail,
  }) {
    final result = create();
    if (patterns != null) result.patterns.addAll(patterns);
    if (totalPass != null) result.totalPass = totalPass;
    if (totalFail != null) result.totalFail = totalFail;
    return result;
  }

  MultiPatternReport._();

  factory MultiPatternReport.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MultiPatternReport.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MultiPatternReport',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'observability_cascade.v1'),
      createEmptyInstance: create)
    ..pPM<CascadeReport>(1, _omitFieldNames ? '' : 'patterns',
        subBuilder: CascadeReport.create)
    ..aI(2, _omitFieldNames ? '' : 'totalPass')
    ..aI(3, _omitFieldNames ? '' : 'totalFail')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MultiPatternReport clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MultiPatternReport copyWith(void Function(MultiPatternReport) updates) =>
      super.copyWith((message) => updates(message as MultiPatternReport))
          as MultiPatternReport;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MultiPatternReport create() => MultiPatternReport._();
  @$core.override
  MultiPatternReport createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MultiPatternReport getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MultiPatternReport>(create);
  static MultiPatternReport? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<CascadeReport> get patterns => $_getList(0);

  @$pb.TagNumber(2)
  $core.int get totalPass => $_getIZ(1);
  @$pb.TagNumber(2)
  set totalPass($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTotalPass() => $_has(1);
  @$pb.TagNumber(2)
  void clearTotalPass() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get totalFail => $_getIZ(2);
  @$pb.TagNumber(3)
  set totalFail($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalFail() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalFail() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
