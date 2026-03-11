// This is a generated file - do not edit.
//
// Generated from greeting/v1/greeting.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class ListLanguagesRequest extends $pb.GeneratedMessage {
  factory ListLanguagesRequest() => create();

  ListLanguagesRequest._();

  factory ListLanguagesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListLanguagesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListLanguagesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLanguagesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLanguagesRequest copyWith(void Function(ListLanguagesRequest) updates) =>
      super.copyWith((message) => updates(message as ListLanguagesRequest))
          as ListLanguagesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListLanguagesRequest create() => ListLanguagesRequest._();
  @$core.override
  ListLanguagesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListLanguagesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListLanguagesRequest>(create);
  static ListLanguagesRequest? _defaultInstance;
}

class ListLanguagesResponse extends $pb.GeneratedMessage {
  factory ListLanguagesResponse({
    $core.Iterable<Language>? languages,
  }) {
    final result = create();
    if (languages != null) result.languages.addAll(languages);
    return result;
  }

  ListLanguagesResponse._();

  factory ListLanguagesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListLanguagesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListLanguagesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..pPM<Language>(1, _omitFieldNames ? '' : 'languages',
        subBuilder: Language.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLanguagesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLanguagesResponse copyWith(
          void Function(ListLanguagesResponse) updates) =>
      super.copyWith((message) => updates(message as ListLanguagesResponse))
          as ListLanguagesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListLanguagesResponse create() => ListLanguagesResponse._();
  @$core.override
  ListLanguagesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListLanguagesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListLanguagesResponse>(create);
  static ListLanguagesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Language> get languages => $_getList(0);
}

class Language extends $pb.GeneratedMessage {
  factory Language({
    $core.String? code,
    $core.String? name,
    $core.String? native,
  }) {
    final result = create();
    if (code != null) result.code = code;
    if (name != null) result.name = name;
    if (native != null) result.native = native;
    return result;
  }

  Language._();

  factory Language.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Language.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Language',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'native')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Language clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Language copyWith(void Function(Language) updates) =>
      super.copyWith((message) => updates(message as Language)) as Language;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Language create() => Language._();
  @$core.override
  Language createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Language getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Language>(create);
  static Language? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get native => $_getSZ(2);
  @$pb.TagNumber(3)
  set native($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNative() => $_has(2);
  @$pb.TagNumber(3)
  void clearNative() => $_clearField(3);
}

class SayHelloRequest extends $pb.GeneratedMessage {
  factory SayHelloRequest({
    $core.String? name,
    $core.String? langCode,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (langCode != null) result.langCode = langCode;
    return result;
  }

  SayHelloRequest._();

  factory SayHelloRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SayHelloRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SayHelloRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'langCode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SayHelloRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SayHelloRequest copyWith(void Function(SayHelloRequest) updates) =>
      super.copyWith((message) => updates(message as SayHelloRequest))
          as SayHelloRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SayHelloRequest create() => SayHelloRequest._();
  @$core.override
  SayHelloRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SayHelloRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SayHelloRequest>(create);
  static SayHelloRequest? _defaultInstance;

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

class SayHelloResponse extends $pb.GeneratedMessage {
  factory SayHelloResponse({
    $core.String? greeting,
    $core.String? language,
    $core.String? langCode,
  }) {
    final result = create();
    if (greeting != null) result.greeting = greeting;
    if (language != null) result.language = language;
    if (langCode != null) result.langCode = langCode;
    return result;
  }

  SayHelloResponse._();

  factory SayHelloResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SayHelloResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SayHelloResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'greeting.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'greeting')
    ..aOS(2, _omitFieldNames ? '' : 'language')
    ..aOS(3, _omitFieldNames ? '' : 'langCode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SayHelloResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SayHelloResponse copyWith(void Function(SayHelloResponse) updates) =>
      super.copyWith((message) => updates(message as SayHelloResponse))
          as SayHelloResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SayHelloResponse create() => SayHelloResponse._();
  @$core.override
  SayHelloResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SayHelloResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SayHelloResponse>(create);
  static SayHelloResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get greeting => $_getSZ(0);
  @$pb.TagNumber(1)
  set greeting($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGreeting() => $_has(0);
  @$pb.TagNumber(1)
  void clearGreeting() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get language => $_getSZ(1);
  @$pb.TagNumber(2)
  set language($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLanguage() => $_has(1);
  @$pb.TagNumber(2)
  void clearLanguage() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get langCode => $_getSZ(2);
  @$pb.TagNumber(3)
  set langCode($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLangCode() => $_has(2);
  @$pb.TagNumber(3)
  void clearLangCode() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
