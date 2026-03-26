// This is a generated file - do not edit.
//
// Generated from holons/v1/describe.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'describe.pbenum.dart';
import 'manifest.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'describe.pbenum.dart';

class DescribeRequest extends $pb.GeneratedMessage {
  factory DescribeRequest() => create();

  DescribeRequest._();

  factory DescribeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DescribeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DescribeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DescribeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DescribeRequest copyWith(void Function(DescribeRequest) updates) =>
      super.copyWith((message) => updates(message as DescribeRequest))
          as DescribeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DescribeRequest create() => DescribeRequest._();
  @$core.override
  DescribeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DescribeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DescribeRequest>(create);
  static DescribeRequest? _defaultInstance;
}

class DescribeResponse extends $pb.GeneratedMessage {
  factory DescribeResponse({
    $1.HolonManifest? manifest,
    $core.Iterable<ServiceDoc>? services,
  }) {
    final result = create();
    if (manifest != null) result.manifest = manifest;
    if (services != null) result.services.addAll(services);
    return result;
  }

  DescribeResponse._();

  factory DescribeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DescribeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DescribeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOM<$1.HolonManifest>(1, _omitFieldNames ? '' : 'manifest',
        subBuilder: $1.HolonManifest.create)
    ..pPM<ServiceDoc>(2, _omitFieldNames ? '' : 'services',
        subBuilder: ServiceDoc.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DescribeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DescribeResponse copyWith(void Function(DescribeResponse) updates) =>
      super.copyWith((message) => updates(message as DescribeResponse))
          as DescribeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DescribeResponse create() => DescribeResponse._();
  @$core.override
  DescribeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DescribeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DescribeResponse>(create);
  static DescribeResponse? _defaultInstance;

  /// Full holon manifest (identity, skills, build, guide, etc.).
  @$pb.TagNumber(1)
  $1.HolonManifest get manifest => $_getN(0);
  @$pb.TagNumber(1)
  set manifest($1.HolonManifest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasManifest() => $_has(0);
  @$pb.TagNumber(1)
  void clearManifest() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.HolonManifest ensureManifest() => $_ensure(0);

  /// One entry per gRPC service the holon exposes (excluding HolonMeta itself).
  @$pb.TagNumber(2)
  $pb.PbList<ServiceDoc> get services => $_getList(1);
}

/// ServiceDoc documents a single gRPC service.
class ServiceDoc extends $pb.GeneratedMessage {
  factory ServiceDoc({
    $core.String? name,
    $core.String? description,
    $core.Iterable<MethodDoc>? methods,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (methods != null) result.methods.addAll(methods);
    return result;
  }

  ServiceDoc._();

  factory ServiceDoc.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ServiceDoc.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ServiceDoc',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..pPM<MethodDoc>(3, _omitFieldNames ? '' : 'methods',
        subBuilder: MethodDoc.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ServiceDoc clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ServiceDoc copyWith(void Function(ServiceDoc) updates) =>
      super.copyWith((message) => updates(message as ServiceDoc)) as ServiceDoc;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ServiceDoc create() => ServiceDoc._();
  @$core.override
  ServiceDoc createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ServiceDoc getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ServiceDoc>(create);
  static ServiceDoc? _defaultInstance;

  /// Fully qualified service name, e.g. "greeting.v1.GreetingService".
  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  /// Human-readable description from the proto comment.
  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  /// One entry per RPC method in the service.
  @$pb.TagNumber(3)
  $pb.PbList<MethodDoc> get methods => $_getList(2);
}

/// MethodDoc documents a single RPC method.
class MethodDoc extends $pb.GeneratedMessage {
  factory MethodDoc({
    $core.String? name,
    $core.String? description,
    $core.String? inputType,
    $core.String? outputType,
    $core.Iterable<FieldDoc>? inputFields,
    $core.Iterable<FieldDoc>? outputFields,
    $core.bool? clientStreaming,
    $core.bool? serverStreaming,
    $core.String? exampleInput,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (inputType != null) result.inputType = inputType;
    if (outputType != null) result.outputType = outputType;
    if (inputFields != null) result.inputFields.addAll(inputFields);
    if (outputFields != null) result.outputFields.addAll(outputFields);
    if (clientStreaming != null) result.clientStreaming = clientStreaming;
    if (serverStreaming != null) result.serverStreaming = serverStreaming;
    if (exampleInput != null) result.exampleInput = exampleInput;
    return result;
  }

  MethodDoc._();

  factory MethodDoc.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MethodDoc.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MethodDoc',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOS(3, _omitFieldNames ? '' : 'inputType')
    ..aOS(4, _omitFieldNames ? '' : 'outputType')
    ..pPM<FieldDoc>(5, _omitFieldNames ? '' : 'inputFields',
        subBuilder: FieldDoc.create)
    ..pPM<FieldDoc>(6, _omitFieldNames ? '' : 'outputFields',
        subBuilder: FieldDoc.create)
    ..aOB(7, _omitFieldNames ? '' : 'clientStreaming')
    ..aOB(8, _omitFieldNames ? '' : 'serverStreaming')
    ..aOS(9, _omitFieldNames ? '' : 'exampleInput')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MethodDoc clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MethodDoc copyWith(void Function(MethodDoc) updates) =>
      super.copyWith((message) => updates(message as MethodDoc)) as MethodDoc;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MethodDoc create() => MethodDoc._();
  @$core.override
  MethodDoc createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MethodDoc getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MethodDoc>(create);
  static MethodDoc? _defaultInstance;

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

  /// Fully qualified input/output message types.
  @$pb.TagNumber(3)
  $core.String get inputType => $_getSZ(2);
  @$pb.TagNumber(3)
  set inputType($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasInputType() => $_has(2);
  @$pb.TagNumber(3)
  void clearInputType() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get outputType => $_getSZ(3);
  @$pb.TagNumber(4)
  set outputType($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOutputType() => $_has(3);
  @$pb.TagNumber(4)
  void clearOutputType() => $_clearField(4);

  /// Field documentation for input and output messages.
  @$pb.TagNumber(5)
  $pb.PbList<FieldDoc> get inputFields => $_getList(4);

  @$pb.TagNumber(6)
  $pb.PbList<FieldDoc> get outputFields => $_getList(5);

  @$pb.TagNumber(7)
  $core.bool get clientStreaming => $_getBF(6);
  @$pb.TagNumber(7)
  set clientStreaming($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasClientStreaming() => $_has(6);
  @$pb.TagNumber(7)
  void clearClientStreaming() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get serverStreaming => $_getBF(7);
  @$pb.TagNumber(8)
  set serverStreaming($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasServerStreaming() => $_has(7);
  @$pb.TagNumber(8)
  void clearServerStreaming() => $_clearField(8);

  /// Concrete example request as JSON, from @example tags in proto comments.
  @$pb.TagNumber(9)
  $core.String get exampleInput => $_getSZ(8);
  @$pb.TagNumber(9)
  set exampleInput($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasExampleInput() => $_has(8);
  @$pb.TagNumber(9)
  void clearExampleInput() => $_clearField(9);
}

/// FieldDoc documents a single message field.
class FieldDoc extends $pb.GeneratedMessage {
  factory FieldDoc({
    $core.String? name,
    $core.String? type,
    $core.int? number,
    $core.String? description,
    FieldLabel? label,
    $core.String? mapKeyType,
    $core.String? mapValueType,
    $core.Iterable<FieldDoc>? nestedFields,
    $core.Iterable<EnumValueDoc>? enumValues,
    $core.bool? required,
    $core.String? example,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (type != null) result.type = type;
    if (number != null) result.number = number;
    if (description != null) result.description = description;
    if (label != null) result.label = label;
    if (mapKeyType != null) result.mapKeyType = mapKeyType;
    if (mapValueType != null) result.mapValueType = mapValueType;
    if (nestedFields != null) result.nestedFields.addAll(nestedFields);
    if (enumValues != null) result.enumValues.addAll(enumValues);
    if (required != null) result.required = required;
    if (example != null) result.example = example;
    return result;
  }

  FieldDoc._();

  factory FieldDoc.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FieldDoc.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FieldDoc',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'type')
    ..aI(3, _omitFieldNames ? '' : 'number')
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..aE<FieldLabel>(5, _omitFieldNames ? '' : 'label',
        enumValues: FieldLabel.values)
    ..aOS(6, _omitFieldNames ? '' : 'mapKeyType')
    ..aOS(7, _omitFieldNames ? '' : 'mapValueType')
    ..pPM<FieldDoc>(8, _omitFieldNames ? '' : 'nestedFields',
        subBuilder: FieldDoc.create)
    ..pPM<EnumValueDoc>(9, _omitFieldNames ? '' : 'enumValues',
        subBuilder: EnumValueDoc.create)
    ..aOB(10, _omitFieldNames ? '' : 'required')
    ..aOS(11, _omitFieldNames ? '' : 'example')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FieldDoc clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FieldDoc copyWith(void Function(FieldDoc) updates) =>
      super.copyWith((message) => updates(message as FieldDoc)) as FieldDoc;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FieldDoc create() => FieldDoc._();
  @$core.override
  FieldDoc createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FieldDoc getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<FieldDoc>(create);
  static FieldDoc? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get type => $_getSZ(1);
  @$pb.TagNumber(2)
  set type($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get number => $_getIZ(2);
  @$pb.TagNumber(3)
  set number($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNumber() => $_has(2);
  @$pb.TagNumber(3)
  void clearNumber() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  @$pb.TagNumber(5)
  FieldLabel get label => $_getN(4);
  @$pb.TagNumber(5)
  set label(FieldLabel value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasLabel() => $_has(4);
  @$pb.TagNumber(5)
  void clearLabel() => $_clearField(5);

  /// Map-specific type information.
  @$pb.TagNumber(6)
  $core.String get mapKeyType => $_getSZ(5);
  @$pb.TagNumber(6)
  set mapKeyType($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasMapKeyType() => $_has(5);
  @$pb.TagNumber(6)
  void clearMapKeyType() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get mapValueType => $_getSZ(6);
  @$pb.TagNumber(7)
  set mapValueType($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasMapValueType() => $_has(6);
  @$pb.TagNumber(7)
  void clearMapValueType() => $_clearField(7);

  /// Recursive: nested message fields or enum values.
  @$pb.TagNumber(8)
  $pb.PbList<FieldDoc> get nestedFields => $_getList(7);

  @$pb.TagNumber(9)
  $pb.PbList<EnumValueDoc> get enumValues => $_getList(8);

  /// Semantic metadata from @required and @example proto comment tags.
  @$pb.TagNumber(10)
  $core.bool get required => $_getBF(9);
  @$pb.TagNumber(10)
  set required($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasRequired() => $_has(9);
  @$pb.TagNumber(10)
  void clearRequired() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get example => $_getSZ(10);
  @$pb.TagNumber(11)
  set example($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasExample() => $_has(10);
  @$pb.TagNumber(11)
  void clearExample() => $_clearField(11);
}

/// EnumValueDoc documents a single enum value.
class EnumValueDoc extends $pb.GeneratedMessage {
  factory EnumValueDoc({
    $core.String? name,
    $core.int? number,
    $core.String? description,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (number != null) result.number = number;
    if (description != null) result.description = description;
    return result;
  }

  EnumValueDoc._();

  factory EnumValueDoc.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EnumValueDoc.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EnumValueDoc',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aI(2, _omitFieldNames ? '' : 'number')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnumValueDoc clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnumValueDoc copyWith(void Function(EnumValueDoc) updates) =>
      super.copyWith((message) => updates(message as EnumValueDoc))
          as EnumValueDoc;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EnumValueDoc create() => EnumValueDoc._();
  @$core.override
  EnumValueDoc createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EnumValueDoc getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EnumValueDoc>(create);
  static EnumValueDoc? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get number => $_getIZ(1);
  @$pb.TagNumber(2)
  set number($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNumber() => $_has(1);
  @$pb.TagNumber(2)
  void clearNumber() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
