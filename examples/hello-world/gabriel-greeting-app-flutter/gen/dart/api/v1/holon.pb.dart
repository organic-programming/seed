// This is a generated file - do not edit.
//
// Generated from api/v1/holon.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class SelectHolonRequest extends $pb.GeneratedMessage {
  factory SelectHolonRequest({
    $core.String? slug,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    return result;
  }

  SelectHolonRequest._();

  factory SelectHolonRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SelectHolonRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SelectHolonRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectHolonRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectHolonRequest copyWith(void Function(SelectHolonRequest) updates) =>
      super.copyWith((message) => updates(message as SelectHolonRequest))
          as SelectHolonRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SelectHolonRequest create() => SelectHolonRequest._();
  @$core.override
  SelectHolonRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SelectHolonRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SelectHolonRequest>(create);
  static SelectHolonRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);
}

class SelectHolonResponse extends $pb.GeneratedMessage {
  factory SelectHolonResponse({
    $core.String? slug,
    $core.String? displayName,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    if (displayName != null) result.displayName = displayName;
    return result;
  }

  SelectHolonResponse._();

  factory SelectHolonResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SelectHolonResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SelectHolonResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..aOS(2, _omitFieldNames ? '' : 'displayName')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectHolonResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectHolonResponse copyWith(void Function(SelectHolonResponse) updates) =>
      super.copyWith((message) => updates(message as SelectHolonResponse))
          as SelectHolonResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SelectHolonResponse create() => SelectHolonResponse._();
  @$core.override
  SelectHolonResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SelectHolonResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SelectHolonResponse>(create);
  static SelectHolonResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get displayName => $_getSZ(1);
  @$pb.TagNumber(2)
  set displayName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplayName() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplayName() => $_clearField(2);
}

class SelectTransportRequest extends $pb.GeneratedMessage {
  factory SelectTransportRequest({
    $core.String? transport,
  }) {
    final result = create();
    if (transport != null) result.transport = transport;
    return result;
  }

  SelectTransportRequest._();

  factory SelectTransportRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SelectTransportRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SelectTransportRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'transport')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectTransportRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectTransportRequest copyWith(
          void Function(SelectTransportRequest) updates) =>
      super.copyWith((message) => updates(message as SelectTransportRequest))
          as SelectTransportRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SelectTransportRequest create() => SelectTransportRequest._();
  @$core.override
  SelectTransportRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SelectTransportRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SelectTransportRequest>(create);
  static SelectTransportRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get transport => $_getSZ(0);
  @$pb.TagNumber(1)
  set transport($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTransport() => $_has(0);
  @$pb.TagNumber(1)
  void clearTransport() => $_clearField(1);
}

class SelectTransportResponse extends $pb.GeneratedMessage {
  factory SelectTransportResponse({
    $core.String? transport,
  }) {
    final result = create();
    if (transport != null) result.transport = transport;
    return result;
  }

  SelectTransportResponse._();

  factory SelectTransportResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SelectTransportResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SelectTransportResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'transport')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectTransportResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectTransportResponse copyWith(
          void Function(SelectTransportResponse) updates) =>
      super.copyWith((message) => updates(message as SelectTransportResponse))
          as SelectTransportResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SelectTransportResponse create() => SelectTransportResponse._();
  @$core.override
  SelectTransportResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SelectTransportResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SelectTransportResponse>(create);
  static SelectTransportResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get transport => $_getSZ(0);
  @$pb.TagNumber(1)
  set transport($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTransport() => $_has(0);
  @$pb.TagNumber(1)
  void clearTransport() => $_clearField(1);
}

class SelectLanguageRequest extends $pb.GeneratedMessage {
  factory SelectLanguageRequest({
    $core.String? code,
  }) {
    final result = create();
    if (code != null) result.code = code;
    return result;
  }

  SelectLanguageRequest._();

  factory SelectLanguageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SelectLanguageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SelectLanguageRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectLanguageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectLanguageRequest copyWith(
          void Function(SelectLanguageRequest) updates) =>
      super.copyWith((message) => updates(message as SelectLanguageRequest))
          as SelectLanguageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SelectLanguageRequest create() => SelectLanguageRequest._();
  @$core.override
  SelectLanguageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SelectLanguageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SelectLanguageRequest>(create);
  static SelectLanguageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);
}

class SelectLanguageResponse extends $pb.GeneratedMessage {
  factory SelectLanguageResponse({
    $core.String? code,
  }) {
    final result = create();
    if (code != null) result.code = code;
    return result;
  }

  SelectLanguageResponse._();

  factory SelectLanguageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SelectLanguageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SelectLanguageResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectLanguageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectLanguageResponse copyWith(
          void Function(SelectLanguageResponse) updates) =>
      super.copyWith((message) => updates(message as SelectLanguageResponse))
          as SelectLanguageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SelectLanguageResponse create() => SelectLanguageResponse._();
  @$core.override
  SelectLanguageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SelectLanguageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SelectLanguageResponse>(create);
  static SelectLanguageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);
}

class GreetRequest extends $pb.GeneratedMessage {
  factory GreetRequest({
    $core.String? name,
    $core.String? langCode,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (langCode != null) result.langCode = langCode;
    return result;
  }

  GreetRequest._();

  factory GreetRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GreetRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GreetRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'langCode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GreetRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GreetRequest copyWith(void Function(GreetRequest) updates) =>
      super.copyWith((message) => updates(message as GreetRequest))
          as GreetRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GreetRequest create() => GreetRequest._();
  @$core.override
  GreetRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GreetRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GreetRequest>(create);
  static GreetRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get langCode => $_getSZ(1);
  @$pb.TagNumber(2)
  set langCode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLangCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearLangCode() => $_clearField(2);
}

class GreetResponse extends $pb.GeneratedMessage {
  factory GreetResponse({
    $core.String? greeting,
  }) {
    final result = create();
    if (greeting != null) result.greeting = greeting;
    return result;
  }

  GreetResponse._();

  factory GreetResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GreetResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GreetResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'greeting')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GreetResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GreetResponse copyWith(void Function(GreetResponse) updates) =>
      super.copyWith((message) => updates(message as GreetResponse))
          as GreetResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GreetResponse create() => GreetResponse._();
  @$core.override
  GreetResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GreetResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GreetResponse>(create);
  static GreetResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get greeting => $_getSZ(0);
  @$pb.TagNumber(1)
  set greeting($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGreeting() => $_has(0);
  @$pb.TagNumber(1)
  void clearGreeting() => $_clearField(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
