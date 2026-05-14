// This is a generated file - do not edit.
//
// Generated from relay/v1/relay.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class TickRequest extends $pb.GeneratedMessage {
  factory TickRequest({
    $core.String? sender,
    $core.String? note,
  }) {
    final result = create();
    if (sender != null) result.sender = sender;
    if (note != null) result.note = note;
    return result;
  }

  TickRequest._();

  factory TickRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TickRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TickRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'relay.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sender')
    ..aOS(2, _omitFieldNames ? '' : 'note')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TickRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TickRequest copyWith(void Function(TickRequest) updates) =>
      super.copyWith((message) => updates(message as TickRequest))
          as TickRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TickRequest create() => TickRequest._();
  @$core.override
  TickRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TickRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TickRequest>(create);
  static TickRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sender => $_getSZ(0);
  @$pb.TagNumber(1)
  set sender($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSender() => $_has(0);
  @$pb.TagNumber(1)
  void clearSender() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get note => $_getSZ(1);
  @$pb.TagNumber(2)
  set note($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNote() => $_has(1);
  @$pb.TagNumber(2)
  void clearNote() => $_clearField(2);
}

class TickResponse extends $pb.GeneratedMessage {
  factory TickResponse({
    $core.String? responderSlug,
    $core.String? responderInstanceUid,
  }) {
    final result = create();
    if (responderSlug != null) result.responderSlug = responderSlug;
    if (responderInstanceUid != null)
      result.responderInstanceUid = responderInstanceUid;
    return result;
  }

  TickResponse._();

  factory TickResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TickResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TickResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'relay.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'responderSlug')
    ..aOS(2, _omitFieldNames ? '' : 'responderInstanceUid')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TickResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TickResponse copyWith(void Function(TickResponse) updates) =>
      super.copyWith((message) => updates(message as TickResponse))
          as TickResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TickResponse create() => TickResponse._();
  @$core.override
  TickResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TickResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TickResponse>(create);
  static TickResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get responderSlug => $_getSZ(0);
  @$pb.TagNumber(1)
  set responderSlug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasResponderSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearResponderSlug() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get responderInstanceUid => $_getSZ(1);
  @$pb.TagNumber(2)
  set responderInstanceUid($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasResponderInstanceUid() => $_has(1);
  @$pb.TagNumber(2)
  void clearResponderInstanceUid() => $_clearField(2);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
