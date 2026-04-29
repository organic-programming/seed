// This is a generated file - do not edit.
//
// Generated from holons/v1/manifest.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'manifest.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'manifest.pbenum.dart';

class HolonManifest_Identity extends $pb.GeneratedMessage {
  factory HolonManifest_Identity({
    $core.String? schema,
    $core.String? uuid,
    $core.String? givenName,
    $core.String? familyName,
    $core.String? motto,
    $core.String? composer,
    $core.String? status,
    $core.String? born,
    $core.String? version,
    $core.Iterable<$core.String>? aliases,
  }) {
    final result = create();
    if (schema != null) result.schema = schema;
    if (uuid != null) result.uuid = uuid;
    if (givenName != null) result.givenName = givenName;
    if (familyName != null) result.familyName = familyName;
    if (motto != null) result.motto = motto;
    if (composer != null) result.composer = composer;
    if (status != null) result.status = status;
    if (born != null) result.born = born;
    if (version != null) result.version = version;
    if (aliases != null) result.aliases.addAll(aliases);
    return result;
  }

  HolonManifest_Identity._();

  factory HolonManifest_Identity.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Identity.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Identity',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'schema')
    ..aOS(2, _omitFieldNames ? '' : 'uuid')
    ..aOS(3, _omitFieldNames ? '' : 'givenName')
    ..aOS(4, _omitFieldNames ? '' : 'familyName')
    ..aOS(5, _omitFieldNames ? '' : 'motto')
    ..aOS(6, _omitFieldNames ? '' : 'composer')
    ..aOS(8, _omitFieldNames ? '' : 'status')
    ..aOS(9, _omitFieldNames ? '' : 'born')
    ..aOS(10, _omitFieldNames ? '' : 'version')
    ..pPS(11, _omitFieldNames ? '' : 'aliases')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Identity clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Identity copyWith(
          void Function(HolonManifest_Identity) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Identity))
          as HolonManifest_Identity;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Identity create() => HolonManifest_Identity._();
  @$core.override
  HolonManifest_Identity createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Identity getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Identity>(create);
  static HolonManifest_Identity? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get schema => $_getSZ(0);
  @$pb.TagNumber(1)
  set schema($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSchema() => $_has(0);
  @$pb.TagNumber(1)
  void clearSchema() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get uuid => $_getSZ(1);
  @$pb.TagNumber(2)
  set uuid($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUuid() => $_has(1);
  @$pb.TagNumber(2)
  void clearUuid() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get givenName => $_getSZ(2);
  @$pb.TagNumber(3)
  set givenName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasGivenName() => $_has(2);
  @$pb.TagNumber(3)
  void clearGivenName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get familyName => $_getSZ(3);
  @$pb.TagNumber(4)
  set familyName($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFamilyName() => $_has(3);
  @$pb.TagNumber(4)
  void clearFamilyName() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get motto => $_getSZ(4);
  @$pb.TagNumber(5)
  set motto($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMotto() => $_has(4);
  @$pb.TagNumber(5)
  void clearMotto() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get composer => $_getSZ(5);
  @$pb.TagNumber(6)
  set composer($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasComposer() => $_has(5);
  @$pb.TagNumber(6)
  void clearComposer() => $_clearField(6);

  @$pb.TagNumber(8)
  $core.String get status => $_getSZ(6);
  @$pb.TagNumber(8)
  set status($core.String value) => $_setString(6, value);
  @$pb.TagNumber(8)
  $core.bool hasStatus() => $_has(6);
  @$pb.TagNumber(8)
  void clearStatus() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get born => $_getSZ(7);
  @$pb.TagNumber(9)
  set born($core.String value) => $_setString(7, value);
  @$pb.TagNumber(9)
  $core.bool hasBorn() => $_has(7);
  @$pb.TagNumber(9)
  void clearBorn() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get version => $_getSZ(8);
  @$pb.TagNumber(10)
  set version($core.String value) => $_setString(8, value);
  @$pb.TagNumber(10)
  $core.bool hasVersion() => $_has(8);
  @$pb.TagNumber(10)
  void clearVersion() => $_clearField(10);

  @$pb.TagNumber(11)
  $pb.PbList<$core.String> get aliases => $_getList(9);
}

class HolonManifest_Skill extends $pb.GeneratedMessage {
  factory HolonManifest_Skill({
    $core.String? name,
    $core.String? description,
    $core.String? when,
    $core.Iterable<$core.String>? steps,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (when != null) result.when = when;
    if (steps != null) result.steps.addAll(steps);
    return result;
  }

  HolonManifest_Skill._();

  factory HolonManifest_Skill.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Skill.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Skill',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOS(3, _omitFieldNames ? '' : 'when')
    ..pPS(4, _omitFieldNames ? '' : 'steps')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Skill clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Skill copyWith(void Function(HolonManifest_Skill) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Skill))
          as HolonManifest_Skill;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Skill create() => HolonManifest_Skill._();
  @$core.override
  HolonManifest_Skill createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Skill getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Skill>(create);
  static HolonManifest_Skill? _defaultInstance;

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
  $core.String get when => $_getSZ(2);
  @$pb.TagNumber(3)
  set when($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWhen() => $_has(2);
  @$pb.TagNumber(3)
  void clearWhen() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get steps => $_getList(3);
}

class HolonManifest_Sequence_Param extends $pb.GeneratedMessage {
  factory HolonManifest_Sequence_Param({
    $core.String? name,
    $core.String? description,
    $core.bool? required,
    $core.String? default_4,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (required != null) result.required = required;
    if (default_4 != null) result.default_4 = default_4;
    return result;
  }

  HolonManifest_Sequence_Param._();

  factory HolonManifest_Sequence_Param.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Sequence_Param.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Sequence.Param',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOB(3, _omitFieldNames ? '' : 'required')
    ..aOS(4, _omitFieldNames ? '' : 'default')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Sequence_Param clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Sequence_Param copyWith(
          void Function(HolonManifest_Sequence_Param) updates) =>
      super.copyWith(
              (message) => updates(message as HolonManifest_Sequence_Param))
          as HolonManifest_Sequence_Param;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Sequence_Param create() =>
      HolonManifest_Sequence_Param._();
  @$core.override
  HolonManifest_Sequence_Param createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Sequence_Param getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Sequence_Param>(create);
  static HolonManifest_Sequence_Param? _defaultInstance;

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
  $core.bool get required => $_getBF(2);
  @$pb.TagNumber(3)
  set required($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRequired() => $_has(2);
  @$pb.TagNumber(3)
  void clearRequired() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get default_4 => $_getSZ(3);
  @$pb.TagNumber(4)
  set default_4($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDefault_4() => $_has(3);
  @$pb.TagNumber(4)
  void clearDefault_4() => $_clearField(4);
}

class HolonManifest_Sequence extends $pb.GeneratedMessage {
  factory HolonManifest_Sequence({
    $core.String? name,
    $core.String? description,
    $core.Iterable<HolonManifest_Sequence_Param>? params,
    $core.Iterable<$core.String>? steps,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (params != null) result.params.addAll(params);
    if (steps != null) result.steps.addAll(steps);
    return result;
  }

  HolonManifest_Sequence._();

  factory HolonManifest_Sequence.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Sequence.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Sequence',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..pPM<HolonManifest_Sequence_Param>(3, _omitFieldNames ? '' : 'params',
        subBuilder: HolonManifest_Sequence_Param.create)
    ..pPS(4, _omitFieldNames ? '' : 'steps')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Sequence clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Sequence copyWith(
          void Function(HolonManifest_Sequence) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Sequence))
          as HolonManifest_Sequence;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Sequence create() => HolonManifest_Sequence._();
  @$core.override
  HolonManifest_Sequence createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Sequence getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Sequence>(create);
  static HolonManifest_Sequence? _defaultInstance;

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
  $pb.PbList<HolonManifest_Sequence_Param> get params => $_getList(2);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get steps => $_getList(3);
}

class HolonManifest_Contract extends $pb.GeneratedMessage {
  factory HolonManifest_Contract({
    $core.String? proto,
    $core.String? service,
    $core.Iterable<$core.String>? rpcs,
  }) {
    final result = create();
    if (proto != null) result.proto = proto;
    if (service != null) result.service = service;
    if (rpcs != null) result.rpcs.addAll(rpcs);
    return result;
  }

  HolonManifest_Contract._();

  factory HolonManifest_Contract.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Contract.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Contract',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'proto')
    ..aOS(2, _omitFieldNames ? '' : 'service')
    ..pPS(3, _omitFieldNames ? '' : 'rpcs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Contract clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Contract copyWith(
          void Function(HolonManifest_Contract) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Contract))
          as HolonManifest_Contract;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Contract create() => HolonManifest_Contract._();
  @$core.override
  HolonManifest_Contract createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Contract getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Contract>(create);
  static HolonManifest_Contract? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get proto => $_getSZ(0);
  @$pb.TagNumber(1)
  set proto($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProto() => $_has(0);
  @$pb.TagNumber(1)
  void clearProto() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get service => $_getSZ(1);
  @$pb.TagNumber(2)
  set service($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasService() => $_has(1);
  @$pb.TagNumber(2)
  void clearService() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get rpcs => $_getList(2);
}

class HolonManifest_Build_Defaults extends $pb.GeneratedMessage {
  factory HolonManifest_Build_Defaults({
    $core.String? target,
    $core.String? mode,
  }) {
    final result = create();
    if (target != null) result.target = target;
    if (mode != null) result.mode = mode;
    return result;
  }

  HolonManifest_Build_Defaults._();

  factory HolonManifest_Build_Defaults.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Build_Defaults.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Build.Defaults',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'target')
    ..aOS(2, _omitFieldNames ? '' : 'mode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build_Defaults clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build_Defaults copyWith(
          void Function(HolonManifest_Build_Defaults) updates) =>
      super.copyWith(
              (message) => updates(message as HolonManifest_Build_Defaults))
          as HolonManifest_Build_Defaults;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build_Defaults create() =>
      HolonManifest_Build_Defaults._();
  @$core.override
  HolonManifest_Build_Defaults createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build_Defaults getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Build_Defaults>(create);
  static HolonManifest_Build_Defaults? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get target => $_getSZ(0);
  @$pb.TagNumber(1)
  set target($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTarget() => $_has(0);
  @$pb.TagNumber(1)
  void clearTarget() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get mode => $_getSZ(1);
  @$pb.TagNumber(2)
  set mode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMode() => $_has(1);
  @$pb.TagNumber(2)
  void clearMode() => $_clearField(2);
}

class HolonManifest_Build_Member extends $pb.GeneratedMessage {
  factory HolonManifest_Build_Member({
    $core.String? id,
    $core.String? path,
    $core.String? type,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (path != null) result.path = path;
    if (type != null) result.type = type;
    return result;
  }

  HolonManifest_Build_Member._();

  factory HolonManifest_Build_Member.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Build_Member.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Build.Member',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'path')
    ..aOS(3, _omitFieldNames ? '' : 'type')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build_Member clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build_Member copyWith(
          void Function(HolonManifest_Build_Member) updates) =>
      super.copyWith(
              (message) => updates(message as HolonManifest_Build_Member))
          as HolonManifest_Build_Member;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build_Member create() => HolonManifest_Build_Member._();
  @$core.override
  HolonManifest_Build_Member createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build_Member getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Build_Member>(create);
  static HolonManifest_Build_Member? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get path => $_getSZ(1);
  @$pb.TagNumber(2)
  set path($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPath() => $_has(1);
  @$pb.TagNumber(2)
  void clearPath() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get type => $_getSZ(2);
  @$pb.TagNumber(3)
  set type($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasType() => $_has(2);
  @$pb.TagNumber(3)
  void clearType() => $_clearField(3);
}

class HolonManifest_Build_Target extends $pb.GeneratedMessage {
  factory HolonManifest_Build_Target({
    $core.Iterable<HolonManifest_Step>? steps,
  }) {
    final result = create();
    if (steps != null) result.steps.addAll(steps);
    return result;
  }

  HolonManifest_Build_Target._();

  factory HolonManifest_Build_Target.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Build_Target.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Build.Target',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPM<HolonManifest_Step>(1, _omitFieldNames ? '' : 'steps',
        subBuilder: HolonManifest_Step.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build_Target clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build_Target copyWith(
          void Function(HolonManifest_Build_Target) updates) =>
      super.copyWith(
              (message) => updates(message as HolonManifest_Build_Target))
          as HolonManifest_Build_Target;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build_Target create() => HolonManifest_Build_Target._();
  @$core.override
  HolonManifest_Build_Target createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build_Target getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Build_Target>(create);
  static HolonManifest_Build_Target? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<HolonManifest_Step> get steps => $_getList(0);
}

class HolonManifest_Build_Codegen extends $pb.GeneratedMessage {
  factory HolonManifest_Build_Codegen({
    $core.Iterable<$core.String>? languages,
  }) {
    final result = create();
    if (languages != null) result.languages.addAll(languages);
    return result;
  }

  HolonManifest_Build_Codegen._();

  factory HolonManifest_Build_Codegen.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Build_Codegen.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Build.Codegen',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'languages')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build_Codegen clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build_Codegen copyWith(
          void Function(HolonManifest_Build_Codegen) updates) =>
      super.copyWith(
              (message) => updates(message as HolonManifest_Build_Codegen))
          as HolonManifest_Build_Codegen;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build_Codegen create() =>
      HolonManifest_Build_Codegen._();
  @$core.override
  HolonManifest_Build_Codegen createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build_Codegen getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Build_Codegen>(create);
  static HolonManifest_Build_Codegen? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get languages => $_getList(0);
}

class HolonManifest_Build extends $pb.GeneratedMessage {
  factory HolonManifest_Build({
    $core.String? runner,
    $core.String? main,
    HolonManifest_Build_Defaults? defaults,
    $core.Iterable<HolonManifest_Build_Member>? members,
    $core.Iterable<$core.MapEntry<$core.String, HolonManifest_Build_Target>>?
        targets,
    $core.Iterable<$core.String>? templates,
    $core.Iterable<HolonManifest_Step_Exec>? beforeCommands,
    $core.Iterable<HolonManifest_Step_Exec>? afterCommands,
    HolonManifest_Build_Codegen? codegen,
  }) {
    final result = create();
    if (runner != null) result.runner = runner;
    if (main != null) result.main = main;
    if (defaults != null) result.defaults = defaults;
    if (members != null) result.members.addAll(members);
    if (targets != null) result.targets.addEntries(targets);
    if (templates != null) result.templates.addAll(templates);
    if (beforeCommands != null) result.beforeCommands.addAll(beforeCommands);
    if (afterCommands != null) result.afterCommands.addAll(afterCommands);
    if (codegen != null) result.codegen = codegen;
    return result;
  }

  HolonManifest_Build._();

  factory HolonManifest_Build.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Build.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Build',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'runner')
    ..aOS(2, _omitFieldNames ? '' : 'main')
    ..aOM<HolonManifest_Build_Defaults>(3, _omitFieldNames ? '' : 'defaults',
        subBuilder: HolonManifest_Build_Defaults.create)
    ..pPM<HolonManifest_Build_Member>(4, _omitFieldNames ? '' : 'members',
        subBuilder: HolonManifest_Build_Member.create)
    ..m<$core.String, HolonManifest_Build_Target>(
        5, _omitFieldNames ? '' : 'targets',
        entryClassName: 'HolonManifest.Build.TargetsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: HolonManifest_Build_Target.create,
        valueDefaultOrMaker: HolonManifest_Build_Target.getDefault,
        packageName: const $pb.PackageName('holons.v1'))
    ..pPS(6, _omitFieldNames ? '' : 'templates')
    ..pPM<HolonManifest_Step_Exec>(7, _omitFieldNames ? '' : 'beforeCommands',
        subBuilder: HolonManifest_Step_Exec.create)
    ..pPM<HolonManifest_Step_Exec>(8, _omitFieldNames ? '' : 'afterCommands',
        subBuilder: HolonManifest_Step_Exec.create)
    ..aOM<HolonManifest_Build_Codegen>(9, _omitFieldNames ? '' : 'codegen',
        subBuilder: HolonManifest_Build_Codegen.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Build copyWith(void Function(HolonManifest_Build) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Build))
          as HolonManifest_Build;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build create() => HolonManifest_Build._();
  @$core.override
  HolonManifest_Build createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Build getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Build>(create);
  static HolonManifest_Build? _defaultInstance;

  /// Compiled:    go-module | cargo | cmake | swift-package
  /// Interpreted: python | node | ruby
  /// Transpiled:  typescript (source is TS, dist is JS)
  /// Mobile:      dart | flutter
  /// Composite:   recipe (multi-step orchestration)
  /// None:        none (no build step, pre-built or external)
  @$pb.TagNumber(1)
  $core.String get runner => $_getSZ(0);
  @$pb.TagNumber(1)
  set runner($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunner() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunner() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get main => $_getSZ(1);
  @$pb.TagNumber(2)
  set main($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMain() => $_has(1);
  @$pb.TagNumber(2)
  void clearMain() => $_clearField(2);

  /// Recipe-mode fields (runner == "recipe")
  @$pb.TagNumber(3)
  HolonManifest_Build_Defaults get defaults => $_getN(2);
  @$pb.TagNumber(3)
  set defaults(HolonManifest_Build_Defaults value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasDefaults() => $_has(2);
  @$pb.TagNumber(3)
  void clearDefaults() => $_clearField(3);
  @$pb.TagNumber(3)
  HolonManifest_Build_Defaults ensureDefaults() => $_ensure(2);

  @$pb.TagNumber(4)
  $pb.PbList<HolonManifest_Build_Member> get members => $_getList(3);

  @$pb.TagNumber(5)
  $pb.PbMap<$core.String, HolonManifest_Build_Target> get targets =>
      $_getMap(4);

  /// Source files containing Go template expressions (e.g. {{ .Version }}).
  /// Resolved with identity data before the language build, restored after.
  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get templates => $_getList(5);

  /// Commands to execute sequentially BEFORE the main runner logic.
  @$pb.TagNumber(7)
  $pb.PbList<HolonManifest_Step_Exec> get beforeCommands => $_getList(6);

  /// Commands to execute sequentially AFTER the main runner logic.
  @$pb.TagNumber(8)
  $pb.PbList<HolonManifest_Step_Exec> get afterCommands => $_getList(7);

  /// Proto code generation languages to run after descriptor production.
  @$pb.TagNumber(9)
  HolonManifest_Build_Codegen get codegen => $_getN(8);
  @$pb.TagNumber(9)
  set codegen(HolonManifest_Build_Codegen value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCodegen() => $_has(8);
  @$pb.TagNumber(9)
  void clearCodegen() => $_clearField(9);
  @$pb.TagNumber(9)
  HolonManifest_Build_Codegen ensureCodegen() => $_ensure(8);
}

class HolonManifest_Step_Exec extends $pb.GeneratedMessage {
  factory HolonManifest_Step_Exec({
    $core.String? cwd,
    $core.Iterable<$core.String>? argv,
  }) {
    final result = create();
    if (cwd != null) result.cwd = cwd;
    if (argv != null) result.argv.addAll(argv);
    return result;
  }

  HolonManifest_Step_Exec._();

  factory HolonManifest_Step_Exec.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Step_Exec.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Step.Exec',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'cwd')
    ..pPS(2, _omitFieldNames ? '' : 'argv')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_Exec clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_Exec copyWith(
          void Function(HolonManifest_Step_Exec) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Step_Exec))
          as HolonManifest_Step_Exec;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_Exec create() => HolonManifest_Step_Exec._();
  @$core.override
  HolonManifest_Step_Exec createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_Exec getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Step_Exec>(create);
  static HolonManifest_Step_Exec? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get cwd => $_getSZ(0);
  @$pb.TagNumber(1)
  set cwd($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCwd() => $_has(0);
  @$pb.TagNumber(1)
  void clearCwd() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get argv => $_getList(1);
}

class HolonManifest_Step_Copy extends $pb.GeneratedMessage {
  factory HolonManifest_Step_Copy({
    $core.String? from,
    $core.String? to,
  }) {
    final result = create();
    if (from != null) result.from = from;
    if (to != null) result.to = to;
    return result;
  }

  HolonManifest_Step_Copy._();

  factory HolonManifest_Step_Copy.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Step_Copy.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Step.Copy',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'from')
    ..aOS(2, _omitFieldNames ? '' : 'to')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_Copy clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_Copy copyWith(
          void Function(HolonManifest_Step_Copy) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Step_Copy))
          as HolonManifest_Step_Copy;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_Copy create() => HolonManifest_Step_Copy._();
  @$core.override
  HolonManifest_Step_Copy createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_Copy getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Step_Copy>(create);
  static HolonManifest_Step_Copy? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get from => $_getSZ(0);
  @$pb.TagNumber(1)
  set from($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFrom() => $_has(0);
  @$pb.TagNumber(1)
  void clearFrom() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get to => $_getSZ(1);
  @$pb.TagNumber(2)
  set to($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTo() => $_has(1);
  @$pb.TagNumber(2)
  void clearTo() => $_clearField(2);
}

class HolonManifest_Step_AssertFile extends $pb.GeneratedMessage {
  factory HolonManifest_Step_AssertFile({
    $core.String? path,
  }) {
    final result = create();
    if (path != null) result.path = path;
    return result;
  }

  HolonManifest_Step_AssertFile._();

  factory HolonManifest_Step_AssertFile.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Step_AssertFile.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Step.AssertFile',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_AssertFile clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_AssertFile copyWith(
          void Function(HolonManifest_Step_AssertFile) updates) =>
      super.copyWith(
              (message) => updates(message as HolonManifest_Step_AssertFile))
          as HolonManifest_Step_AssertFile;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_AssertFile create() =>
      HolonManifest_Step_AssertFile._();
  @$core.override
  HolonManifest_Step_AssertFile createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_AssertFile getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Step_AssertFile>(create);
  static HolonManifest_Step_AssertFile? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);
}

class HolonManifest_Step_CopyArtifact extends $pb.GeneratedMessage {
  factory HolonManifest_Step_CopyArtifact({
    $core.String? from,
    $core.String? to,
  }) {
    final result = create();
    if (from != null) result.from = from;
    if (to != null) result.to = to;
    return result;
  }

  HolonManifest_Step_CopyArtifact._();

  factory HolonManifest_Step_CopyArtifact.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Step_CopyArtifact.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Step.CopyArtifact',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'from')
    ..aOS(2, _omitFieldNames ? '' : 'to')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_CopyArtifact clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_CopyArtifact copyWith(
          void Function(HolonManifest_Step_CopyArtifact) updates) =>
      super.copyWith(
              (message) => updates(message as HolonManifest_Step_CopyArtifact))
          as HolonManifest_Step_CopyArtifact;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_CopyArtifact create() =>
      HolonManifest_Step_CopyArtifact._();
  @$core.override
  HolonManifest_Step_CopyArtifact createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_CopyArtifact getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Step_CopyArtifact>(
          create);
  static HolonManifest_Step_CopyArtifact? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get from => $_getSZ(0);
  @$pb.TagNumber(1)
  set from($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFrom() => $_has(0);
  @$pb.TagNumber(1)
  void clearFrom() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get to => $_getSZ(1);
  @$pb.TagNumber(2)
  set to($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTo() => $_has(1);
  @$pb.TagNumber(2)
  void clearTo() => $_clearField(2);
}

class HolonManifest_Step_CopyAllHolons extends $pb.GeneratedMessage {
  factory HolonManifest_Step_CopyAllHolons({
    $core.String? to,
  }) {
    final result = create();
    if (to != null) result.to = to;
    return result;
  }

  HolonManifest_Step_CopyAllHolons._();

  factory HolonManifest_Step_CopyAllHolons.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Step_CopyAllHolons.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Step.CopyAllHolons',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'to')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_CopyAllHolons clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step_CopyAllHolons copyWith(
          void Function(HolonManifest_Step_CopyAllHolons) updates) =>
      super.copyWith(
              (message) => updates(message as HolonManifest_Step_CopyAllHolons))
          as HolonManifest_Step_CopyAllHolons;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_CopyAllHolons create() =>
      HolonManifest_Step_CopyAllHolons._();
  @$core.override
  HolonManifest_Step_CopyAllHolons createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step_CopyAllHolons getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Step_CopyAllHolons>(
          create);
  static HolonManifest_Step_CopyAllHolons? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get to => $_getSZ(0);
  @$pb.TagNumber(1)
  set to($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTo() => $_has(0);
  @$pb.TagNumber(1)
  void clearTo() => $_clearField(1);
}

enum HolonManifest_Step_Action {
  exec,
  copy,
  buildMember,
  assertFile,
  copyArtifact,
  copyAllHolons,
  notSet
}

class HolonManifest_Step extends $pb.GeneratedMessage {
  factory HolonManifest_Step({
    HolonManifest_Step_Exec? exec,
    HolonManifest_Step_Copy? copy,
    $core.String? buildMember,
    HolonManifest_Step_AssertFile? assertFile,
    HolonManifest_Step_CopyArtifact? copyArtifact,
    HolonManifest_Step_CopyAllHolons? copyAllHolons,
  }) {
    final result = create();
    if (exec != null) result.exec = exec;
    if (copy != null) result.copy = copy;
    if (buildMember != null) result.buildMember = buildMember;
    if (assertFile != null) result.assertFile = assertFile;
    if (copyArtifact != null) result.copyArtifact = copyArtifact;
    if (copyAllHolons != null) result.copyAllHolons = copyAllHolons;
    return result;
  }

  HolonManifest_Step._();

  factory HolonManifest_Step.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Step.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, HolonManifest_Step_Action>
      _HolonManifest_Step_ActionByTag = {
    1: HolonManifest_Step_Action.exec,
    2: HolonManifest_Step_Action.copy,
    3: HolonManifest_Step_Action.buildMember,
    4: HolonManifest_Step_Action.assertFile,
    5: HolonManifest_Step_Action.copyArtifact,
    6: HolonManifest_Step_Action.copyAllHolons,
    0: HolonManifest_Step_Action.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Step',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..oo(0, [1, 2, 3, 4, 5, 6])
    ..aOM<HolonManifest_Step_Exec>(1, _omitFieldNames ? '' : 'exec',
        subBuilder: HolonManifest_Step_Exec.create)
    ..aOM<HolonManifest_Step_Copy>(2, _omitFieldNames ? '' : 'copy',
        subBuilder: HolonManifest_Step_Copy.create)
    ..aOS(3, _omitFieldNames ? '' : 'buildMember')
    ..aOM<HolonManifest_Step_AssertFile>(4, _omitFieldNames ? '' : 'assertFile',
        subBuilder: HolonManifest_Step_AssertFile.create)
    ..aOM<HolonManifest_Step_CopyArtifact>(
        5, _omitFieldNames ? '' : 'copyArtifact',
        subBuilder: HolonManifest_Step_CopyArtifact.create)
    ..aOM<HolonManifest_Step_CopyAllHolons>(
        6, _omitFieldNames ? '' : 'copyAllHolons',
        subBuilder: HolonManifest_Step_CopyAllHolons.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Step copyWith(void Function(HolonManifest_Step) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Step))
          as HolonManifest_Step;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step create() => HolonManifest_Step._();
  @$core.override
  HolonManifest_Step createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Step getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Step>(create);
  static HolonManifest_Step? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  @$pb.TagNumber(6)
  HolonManifest_Step_Action whichAction() =>
      _HolonManifest_Step_ActionByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  @$pb.TagNumber(6)
  void clearAction() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  HolonManifest_Step_Exec get exec => $_getN(0);
  @$pb.TagNumber(1)
  set exec(HolonManifest_Step_Exec value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasExec() => $_has(0);
  @$pb.TagNumber(1)
  void clearExec() => $_clearField(1);
  @$pb.TagNumber(1)
  HolonManifest_Step_Exec ensureExec() => $_ensure(0);

  @$pb.TagNumber(2)
  HolonManifest_Step_Copy get copy => $_getN(1);
  @$pb.TagNumber(2)
  set copy(HolonManifest_Step_Copy value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCopy() => $_has(1);
  @$pb.TagNumber(2)
  void clearCopy() => $_clearField(2);
  @$pb.TagNumber(2)
  HolonManifest_Step_Copy ensureCopy() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get buildMember => $_getSZ(2);
  @$pb.TagNumber(3)
  set buildMember($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBuildMember() => $_has(2);
  @$pb.TagNumber(3)
  void clearBuildMember() => $_clearField(3);

  @$pb.TagNumber(4)
  HolonManifest_Step_AssertFile get assertFile => $_getN(3);
  @$pb.TagNumber(4)
  set assertFile(HolonManifest_Step_AssertFile value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasAssertFile() => $_has(3);
  @$pb.TagNumber(4)
  void clearAssertFile() => $_clearField(4);
  @$pb.TagNumber(4)
  HolonManifest_Step_AssertFile ensureAssertFile() => $_ensure(3);

  @$pb.TagNumber(5)
  HolonManifest_Step_CopyArtifact get copyArtifact => $_getN(4);
  @$pb.TagNumber(5)
  set copyArtifact(HolonManifest_Step_CopyArtifact value) =>
      $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasCopyArtifact() => $_has(4);
  @$pb.TagNumber(5)
  void clearCopyArtifact() => $_clearField(5);
  @$pb.TagNumber(5)
  HolonManifest_Step_CopyArtifact ensureCopyArtifact() => $_ensure(4);

  @$pb.TagNumber(6)
  HolonManifest_Step_CopyAllHolons get copyAllHolons => $_getN(5);
  @$pb.TagNumber(6)
  set copyAllHolons(HolonManifest_Step_CopyAllHolons value) =>
      $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCopyAllHolons() => $_has(5);
  @$pb.TagNumber(6)
  void clearCopyAllHolons() => $_clearField(6);
  @$pb.TagNumber(6)
  HolonManifest_Step_CopyAllHolons ensureCopyAllHolons() => $_ensure(5);
}

class HolonManifest_Requires extends $pb.GeneratedMessage {
  factory HolonManifest_Requires({
    $core.Iterable<$core.String>? commands,
    $core.Iterable<$core.String>? files,
    $core.Iterable<$core.String>? platforms,
    $core.Iterable<$core.String>? sdkPrebuilts,
  }) {
    final result = create();
    if (commands != null) result.commands.addAll(commands);
    if (files != null) result.files.addAll(files);
    if (platforms != null) result.platforms.addAll(platforms);
    if (sdkPrebuilts != null) result.sdkPrebuilts.addAll(sdkPrebuilts);
    return result;
  }

  HolonManifest_Requires._();

  factory HolonManifest_Requires.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Requires.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Requires',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'commands')
    ..pPS(2, _omitFieldNames ? '' : 'files')
    ..pPS(3, _omitFieldNames ? '' : 'platforms')
    ..pPS(4, _omitFieldNames ? '' : 'sdkPrebuilts')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Requires clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Requires copyWith(
          void Function(HolonManifest_Requires) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Requires))
          as HolonManifest_Requires;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Requires create() => HolonManifest_Requires._();
  @$core.override
  HolonManifest_Requires createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Requires getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Requires>(create);
  static HolonManifest_Requires? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get commands => $_getList(0);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get files => $_getList(1);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get platforms => $_getList(2);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get sdkPrebuilts => $_getList(3);
}

class HolonManifest_Artifacts_TargetArtifacts extends $pb.GeneratedMessage {
  factory HolonManifest_Artifacts_TargetArtifacts({
    $core.String? debug,
    $core.String? release,
    $core.String? profile,
  }) {
    final result = create();
    if (debug != null) result.debug = debug;
    if (release != null) result.release = release;
    if (profile != null) result.profile = profile;
    return result;
  }

  HolonManifest_Artifacts_TargetArtifacts._();

  factory HolonManifest_Artifacts_TargetArtifacts.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Artifacts_TargetArtifacts.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Artifacts.TargetArtifacts',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'debug')
    ..aOS(2, _omitFieldNames ? '' : 'release')
    ..aOS(3, _omitFieldNames ? '' : 'profile')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Artifacts_TargetArtifacts clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Artifacts_TargetArtifacts copyWith(
          void Function(HolonManifest_Artifacts_TargetArtifacts) updates) =>
      super.copyWith((message) =>
              updates(message as HolonManifest_Artifacts_TargetArtifacts))
          as HolonManifest_Artifacts_TargetArtifacts;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Artifacts_TargetArtifacts create() =>
      HolonManifest_Artifacts_TargetArtifacts._();
  @$core.override
  HolonManifest_Artifacts_TargetArtifacts createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Artifacts_TargetArtifacts getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          HolonManifest_Artifacts_TargetArtifacts>(create);
  static HolonManifest_Artifacts_TargetArtifacts? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get debug => $_getSZ(0);
  @$pb.TagNumber(1)
  set debug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDebug() => $_has(0);
  @$pb.TagNumber(1)
  void clearDebug() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get release => $_getSZ(1);
  @$pb.TagNumber(2)
  set release($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRelease() => $_has(1);
  @$pb.TagNumber(2)
  void clearRelease() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get profile => $_getSZ(2);
  @$pb.TagNumber(3)
  set profile($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProfile() => $_has(2);
  @$pb.TagNumber(3)
  void clearProfile() => $_clearField(3);
}

class HolonManifest_Artifacts extends $pb.GeneratedMessage {
  factory HolonManifest_Artifacts({
    $core.String? binary,
    $core.String? primary,
    $core.Iterable<
            $core
            .MapEntry<$core.String, HolonManifest_Artifacts_TargetArtifacts>>?
        byTarget,
  }) {
    final result = create();
    if (binary != null) result.binary = binary;
    if (primary != null) result.primary = primary;
    if (byTarget != null) result.byTarget.addEntries(byTarget);
    return result;
  }

  HolonManifest_Artifacts._();

  factory HolonManifest_Artifacts.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest_Artifacts.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest.Artifacts',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'binary')
    ..aOS(2, _omitFieldNames ? '' : 'primary')
    ..m<$core.String, HolonManifest_Artifacts_TargetArtifacts>(
        3, _omitFieldNames ? '' : 'byTarget',
        entryClassName: 'HolonManifest.Artifacts.ByTargetEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: HolonManifest_Artifacts_TargetArtifacts.create,
        valueDefaultOrMaker: HolonManifest_Artifacts_TargetArtifacts.getDefault,
        packageName: const $pb.PackageName('holons.v1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Artifacts clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest_Artifacts copyWith(
          void Function(HolonManifest_Artifacts) updates) =>
      super.copyWith((message) => updates(message as HolonManifest_Artifacts))
          as HolonManifest_Artifacts;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest_Artifacts create() => HolonManifest_Artifacts._();
  @$core.override
  HolonManifest_Artifacts createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest_Artifacts getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest_Artifacts>(create);
  static HolonManifest_Artifacts? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get binary => $_getSZ(0);
  @$pb.TagNumber(1)
  set binary($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBinary() => $_has(0);
  @$pb.TagNumber(1)
  void clearBinary() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get primary => $_getSZ(1);
  @$pb.TagNumber(2)
  set primary($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPrimary() => $_has(1);
  @$pb.TagNumber(2)
  void clearPrimary() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbMap<$core.String, HolonManifest_Artifacts_TargetArtifacts>
      get byTarget => $_getMap(2);
}

/// HolonManifest is the single source of truth for a holon's identity,
/// contract, skills, documentation, and operational metadata.
/// Any .proto file can carry this manifest via:
///   option (holons.v1.manifest) = { ... };
///
/// See HOLON_PROTO.md for the full authoring specification.
/// See HOLON_PACKAGE.md for how the package is produced from this manifest.
class HolonManifest extends $pb.GeneratedMessage {
  factory HolonManifest({
    HolonManifest_Identity? identity,
    $core.String? description,
    $core.String? lang,
    $core.Iterable<HolonManifest_Skill>? skills,
    HolonManifest_Contract? contract,
    $core.String? kind,
    $core.Iterable<$core.String>? platforms,
    $core.String? transport,
    HolonManifest_Build? build,
    HolonManifest_Requires? requires,
    HolonManifest_Artifacts? artifacts,
    $core.Iterable<HolonManifest_Sequence>? sequences,
    $core.String? guide,
    ObservabilityVisibility? sessionVisibility,
    $core.Iterable<ListenerVisibilityOverride>? sessionVisibilityOverrides,
  }) {
    final result = create();
    if (identity != null) result.identity = identity;
    if (description != null) result.description = description;
    if (lang != null) result.lang = lang;
    if (skills != null) result.skills.addAll(skills);
    if (contract != null) result.contract = contract;
    if (kind != null) result.kind = kind;
    if (platforms != null) result.platforms.addAll(platforms);
    if (transport != null) result.transport = transport;
    if (build != null) result.build = build;
    if (requires != null) result.requires = requires;
    if (artifacts != null) result.artifacts = artifacts;
    if (sequences != null) result.sequences.addAll(sequences);
    if (guide != null) result.guide = guide;
    if (sessionVisibility != null) result.sessionVisibility = sessionVisibility;
    if (sessionVisibilityOverrides != null)
      result.sessionVisibilityOverrides.addAll(sessionVisibilityOverrides);
    return result;
  }

  HolonManifest._();

  factory HolonManifest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HolonManifest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HolonManifest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOM<HolonManifest_Identity>(1, _omitFieldNames ? '' : 'identity',
        subBuilder: HolonManifest_Identity.create)
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOS(4, _omitFieldNames ? '' : 'lang')
    ..pPM<HolonManifest_Skill>(5, _omitFieldNames ? '' : 'skills',
        subBuilder: HolonManifest_Skill.create)
    ..aOM<HolonManifest_Contract>(6, _omitFieldNames ? '' : 'contract',
        subBuilder: HolonManifest_Contract.create)
    ..aOS(7, _omitFieldNames ? '' : 'kind')
    ..pPS(8, _omitFieldNames ? '' : 'platforms')
    ..aOS(9, _omitFieldNames ? '' : 'transport')
    ..aOM<HolonManifest_Build>(10, _omitFieldNames ? '' : 'build',
        subBuilder: HolonManifest_Build.create)
    ..aOM<HolonManifest_Requires>(11, _omitFieldNames ? '' : 'requires',
        subBuilder: HolonManifest_Requires.create)
    ..aOM<HolonManifest_Artifacts>(13, _omitFieldNames ? '' : 'artifacts',
        subBuilder: HolonManifest_Artifacts.create)
    ..pPM<HolonManifest_Sequence>(14, _omitFieldNames ? '' : 'sequences',
        subBuilder: HolonManifest_Sequence.create)
    ..aOS(15, _omitFieldNames ? '' : 'guide')
    ..aE<ObservabilityVisibility>(
        16, _omitFieldNames ? '' : 'sessionVisibility',
        enumValues: ObservabilityVisibility.values)
    ..pPM<ListenerVisibilityOverride>(
        17, _omitFieldNames ? '' : 'sessionVisibilityOverrides',
        subBuilder: ListenerVisibilityOverride.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HolonManifest copyWith(void Function(HolonManifest) updates) =>
      super.copyWith((message) => updates(message as HolonManifest))
          as HolonManifest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HolonManifest create() => HolonManifest._();
  @$core.override
  HolonManifest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HolonManifest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HolonManifest>(create);
  static HolonManifest? _defaultInstance;

  @$pb.TagNumber(1)
  HolonManifest_Identity get identity => $_getN(0);
  @$pb.TagNumber(1)
  set identity(HolonManifest_Identity value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIdentity() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdentity() => $_clearField(1);
  @$pb.TagNumber(1)
  HolonManifest_Identity ensureIdentity() => $_ensure(0);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get lang => $_getSZ(2);
  @$pb.TagNumber(4)
  set lang($core.String value) => $_setString(2, value);
  @$pb.TagNumber(4)
  $core.bool hasLang() => $_has(2);
  @$pb.TagNumber(4)
  void clearLang() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<HolonManifest_Skill> get skills => $_getList(3);

  @$pb.TagNumber(6)
  HolonManifest_Contract get contract => $_getN(4);
  @$pb.TagNumber(6)
  set contract(HolonManifest_Contract value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasContract() => $_has(4);
  @$pb.TagNumber(6)
  void clearContract() => $_clearField(6);
  @$pb.TagNumber(6)
  HolonManifest_Contract ensureContract() => $_ensure(4);

  @$pb.TagNumber(7)
  $core.String get kind => $_getSZ(5);
  @$pb.TagNumber(7)
  set kind($core.String value) => $_setString(5, value);
  @$pb.TagNumber(7)
  $core.bool hasKind() => $_has(5);
  @$pb.TagNumber(7)
  void clearKind() => $_clearField(7);

  @$pb.TagNumber(8)
  $pb.PbList<$core.String> get platforms => $_getList(6);

  @$pb.TagNumber(9)
  $core.String get transport => $_getSZ(7);
  @$pb.TagNumber(9)
  set transport($core.String value) => $_setString(7, value);
  @$pb.TagNumber(9)
  $core.bool hasTransport() => $_has(7);
  @$pb.TagNumber(9)
  void clearTransport() => $_clearField(9);

  @$pb.TagNumber(10)
  HolonManifest_Build get build => $_getN(8);
  @$pb.TagNumber(10)
  set build(HolonManifest_Build value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasBuild() => $_has(8);
  @$pb.TagNumber(10)
  void clearBuild() => $_clearField(10);
  @$pb.TagNumber(10)
  HolonManifest_Build ensureBuild() => $_ensure(8);

  @$pb.TagNumber(11)
  HolonManifest_Requires get requires => $_getN(9);
  @$pb.TagNumber(11)
  set requires(HolonManifest_Requires value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasRequires() => $_has(9);
  @$pb.TagNumber(11)
  void clearRequires() => $_clearField(11);
  @$pb.TagNumber(11)
  HolonManifest_Requires ensureRequires() => $_ensure(9);

  @$pb.TagNumber(13)
  HolonManifest_Artifacts get artifacts => $_getN(10);
  @$pb.TagNumber(13)
  set artifacts(HolonManifest_Artifacts value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasArtifacts() => $_has(10);
  @$pb.TagNumber(13)
  void clearArtifacts() => $_clearField(13);
  @$pb.TagNumber(13)
  HolonManifest_Artifacts ensureArtifacts() => $_ensure(10);

  @$pb.TagNumber(14)
  $pb.PbList<HolonManifest_Sequence> get sequences => $_getList(11);

  @$pb.TagNumber(15)
  $core.String get guide => $_getSZ(12);
  @$pb.TagNumber(15)
  set guide($core.String value) => $_setString(12, value);
  @$pb.TagNumber(15)
  $core.bool hasGuide() => $_has(12);
  @$pb.TagNumber(15)
  void clearGuide() => $_clearField(15);

  /// ── Observability visibility ─────────────────────────
  /// Single dial gating HolonSession.* and HolonObservability.* exposure.
  /// UNSPECIFIED defers to the per-listener default the SDK infers from
  /// the listener's scheme and security mode. See SESSIONS.md §Security
  /// and OBSERVABILITY.md §Security Considerations.
  @$pb.TagNumber(16)
  ObservabilityVisibility get sessionVisibility => $_getN(13);
  @$pb.TagNumber(16)
  set sessionVisibility(ObservabilityVisibility value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasSessionVisibility() => $_has(13);
  @$pb.TagNumber(16)
  void clearSessionVisibility() => $_clearField(16);

  /// Per-listener overrides. Each override's listener_uri must match a
  /// listener declared in the holon's build/serve config.
  @$pb.TagNumber(17)
  $pb.PbList<ListenerVisibilityOverride> get sessionVisibilityOverrides =>
      $_getList(14);
}

class ListenerVisibilityOverride extends $pb.GeneratedMessage {
  factory ListenerVisibilityOverride({
    $core.String? listenerUri,
    ObservabilityVisibility? visibility,
  }) {
    final result = create();
    if (listenerUri != null) result.listenerUri = listenerUri;
    if (visibility != null) result.visibility = visibility;
    return result;
  }

  ListenerVisibilityOverride._();

  factory ListenerVisibilityOverride.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListenerVisibilityOverride.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListenerVisibilityOverride',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'listenerUri')
    ..aE<ObservabilityVisibility>(2, _omitFieldNames ? '' : 'visibility',
        enumValues: ObservabilityVisibility.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListenerVisibilityOverride clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListenerVisibilityOverride copyWith(
          void Function(ListenerVisibilityOverride) updates) =>
      super.copyWith(
              (message) => updates(message as ListenerVisibilityOverride))
          as ListenerVisibilityOverride;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListenerVisibilityOverride create() => ListenerVisibilityOverride._();
  @$core.override
  ListenerVisibilityOverride createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListenerVisibilityOverride getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListenerVisibilityOverride>(create);
  static ListenerVisibilityOverride? _defaultInstance;

  /// Listener URI to apply the override to. Matches a listener declared
  /// in the holon's serve config.
  @$pb.TagNumber(1)
  $core.String get listenerUri => $_getSZ(0);
  @$pb.TagNumber(1)
  set listenerUri($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasListenerUri() => $_has(0);
  @$pb.TagNumber(1)
  void clearListenerUri() => $_clearField(1);

  @$pb.TagNumber(2)
  ObservabilityVisibility get visibility => $_getN(1);
  @$pb.TagNumber(2)
  set visibility(ObservabilityVisibility value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasVisibility() => $_has(1);
  @$pb.TagNumber(2)
  void clearVisibility() => $_clearField(2);
}

class Manifest {
  static final manifest = $pb.Extension<HolonManifest>(
      _omitMessageNames ? '' : 'google.protobuf.FileOptions',
      _omitFieldNames ? '' : 'manifest',
      50000,
      $pb.PbFieldType.OM,
      defaultOrMaker: HolonManifest.getDefault,
      subBuilder: HolonManifest.create);
  static void registerAllExtensions($pb.ExtensionRegistry registry) {
    registry.add(manifest);
  }
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
