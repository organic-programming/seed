// This is a generated file - do not edit.
//
// Generated from holons/v1/coax.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'coax.pbenum.dart';
import 'manifest.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'coax.pbenum.dart';

class ListMembersRequest extends $pb.GeneratedMessage {
  factory ListMembersRequest() => create();

  ListMembersRequest._();

  factory ListMembersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMembersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMembersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersRequest copyWith(void Function(ListMembersRequest) updates) =>
      super.copyWith((message) => updates(message as ListMembersRequest))
          as ListMembersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMembersRequest create() => ListMembersRequest._();
  @$core.override
  ListMembersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMembersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMembersRequest>(create);
  static ListMembersRequest? _defaultInstance;
}

class ListMembersResponse extends $pb.GeneratedMessage {
  factory ListMembersResponse({
    $core.Iterable<MemberInfo>? members,
  }) {
    final result = create();
    if (members != null) result.members.addAll(members);
    return result;
  }

  ListMembersResponse._();

  factory ListMembersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMembersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMembersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..pPM<MemberInfo>(1, _omitFieldNames ? '' : 'members',
        subBuilder: MemberInfo.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersResponse copyWith(void Function(ListMembersResponse) updates) =>
      super.copyWith((message) => updates(message as ListMembersResponse))
          as ListMembersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMembersResponse create() => ListMembersResponse._();
  @$core.override
  ListMembersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMembersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMembersResponse>(create);
  static ListMembersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<MemberInfo> get members => $_getList(0);
}

/// MemberInfo describes a member holon visible to the organism.
class MemberInfo extends $pb.GeneratedMessage {
  factory MemberInfo({
    $core.String? slug,
    $1.HolonManifest_Identity? identity,
    MemberState? state,
    $core.bool? isOrganism,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    if (identity != null) result.identity = identity;
    if (state != null) result.state = state;
    if (isOrganism != null) result.isOrganism = isOrganism;
    return result;
  }

  MemberInfo._();

  factory MemberInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..aOM<$1.HolonManifest_Identity>(2, _omitFieldNames ? '' : 'identity',
        subBuilder: $1.HolonManifest_Identity.create)
    ..aE<MemberState>(3, _omitFieldNames ? '' : 'state',
        enumValues: MemberState.values)
    ..aOB(4, _omitFieldNames ? '' : 'isOrganism')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberInfo copyWith(void Function(MemberInfo) updates) =>
      super.copyWith((message) => updates(message as MemberInfo)) as MemberInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberInfo create() => MemberInfo._();
  @$core.override
  MemberInfo createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberInfo getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberInfo>(create);
  static MemberInfo? _defaultInstance;

  /// The member's slug (used for Tell and Connect).
  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);

  /// Identity from the member's manifest.
  @$pb.TagNumber(2)
  $1.HolonManifest_Identity get identity => $_getN(1);
  @$pb.TagNumber(2)
  set identity($1.HolonManifest_Identity value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasIdentity() => $_has(1);
  @$pb.TagNumber(2)
  void clearIdentity() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.HolonManifest_Identity ensureIdentity() => $_ensure(1);

  /// Current runtime status.
  @$pb.TagNumber(3)
  MemberState get state => $_getN(2);
  @$pb.TagNumber(3)
  set state(MemberState value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasState() => $_has(2);
  @$pb.TagNumber(3)
  void clearState() => $_clearField(3);

  /// Whether this member is itself an organism (recursive COAX).
  @$pb.TagNumber(4)
  $core.bool get isOrganism => $_getBF(3);
  @$pb.TagNumber(4)
  set isOrganism($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIsOrganism() => $_has(3);
  @$pb.TagNumber(4)
  void clearIsOrganism() => $_clearField(4);
}

class MemberStatusRequest extends $pb.GeneratedMessage {
  factory MemberStatusRequest({
    $core.String? slug,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    return result;
  }

  MemberStatusRequest._();

  factory MemberStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberStatusRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberStatusRequest copyWith(void Function(MemberStatusRequest) updates) =>
      super.copyWith((message) => updates(message as MemberStatusRequest))
          as MemberStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberStatusRequest create() => MemberStatusRequest._();
  @$core.override
  MemberStatusRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberStatusRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberStatusRequest>(create);
  static MemberStatusRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);
}

class MemberStatusResponse extends $pb.GeneratedMessage {
  factory MemberStatusResponse({
    MemberInfo? member,
  }) {
    final result = create();
    if (member != null) result.member = member;
    return result;
  }

  MemberStatusResponse._();

  factory MemberStatusResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberStatusResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberStatusResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOM<MemberInfo>(1, _omitFieldNames ? '' : 'member',
        subBuilder: MemberInfo.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberStatusResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberStatusResponse copyWith(void Function(MemberStatusResponse) updates) =>
      super.copyWith((message) => updates(message as MemberStatusResponse))
          as MemberStatusResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberStatusResponse create() => MemberStatusResponse._();
  @$core.override
  MemberStatusResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberStatusResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberStatusResponse>(create);
  static MemberStatusResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MemberInfo get member => $_getN(0);
  @$pb.TagNumber(1)
  set member(MemberInfo value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMember() => $_has(0);
  @$pb.TagNumber(1)
  void clearMember() => $_clearField(1);
  @$pb.TagNumber(1)
  MemberInfo ensureMember() => $_ensure(0);
}

class ConnectMemberRequest extends $pb.GeneratedMessage {
  factory ConnectMemberRequest({
    $core.String? slug,
    $core.String? transport,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    if (transport != null) result.transport = transport;
    return result;
  }

  ConnectMemberRequest._();

  factory ConnectMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ConnectMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConnectMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..aOS(2, _omitFieldNames ? '' : 'transport')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConnectMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConnectMemberRequest copyWith(void Function(ConnectMemberRequest) updates) =>
      super.copyWith((message) => updates(message as ConnectMemberRequest))
          as ConnectMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ConnectMemberRequest create() => ConnectMemberRequest._();
  @$core.override
  ConnectMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ConnectMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConnectMemberRequest>(create);
  static ConnectMemberRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);

  /// Optional transport override (default: organism decides).
  @$pb.TagNumber(2)
  $core.String get transport => $_getSZ(1);
  @$pb.TagNumber(2)
  set transport($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTransport() => $_has(1);
  @$pb.TagNumber(2)
  void clearTransport() => $_clearField(2);
}

class ConnectMemberResponse extends $pb.GeneratedMessage {
  factory ConnectMemberResponse({
    MemberInfo? member,
  }) {
    final result = create();
    if (member != null) result.member = member;
    return result;
  }

  ConnectMemberResponse._();

  factory ConnectMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ConnectMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConnectMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOM<MemberInfo>(1, _omitFieldNames ? '' : 'member',
        subBuilder: MemberInfo.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConnectMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConnectMemberResponse copyWith(
          void Function(ConnectMemberResponse) updates) =>
      super.copyWith((message) => updates(message as ConnectMemberResponse))
          as ConnectMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ConnectMemberResponse create() => ConnectMemberResponse._();
  @$core.override
  ConnectMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ConnectMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConnectMemberResponse>(create);
  static ConnectMemberResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MemberInfo get member => $_getN(0);
  @$pb.TagNumber(1)
  set member(MemberInfo value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMember() => $_has(0);
  @$pb.TagNumber(1)
  void clearMember() => $_clearField(1);
  @$pb.TagNumber(1)
  MemberInfo ensureMember() => $_ensure(0);
}

class DisconnectMemberRequest extends $pb.GeneratedMessage {
  factory DisconnectMemberRequest({
    $core.String? slug,
  }) {
    final result = create();
    if (slug != null) result.slug = slug;
    return result;
  }

  DisconnectMemberRequest._();

  factory DisconnectMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DisconnectMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DisconnectMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'slug')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DisconnectMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DisconnectMemberRequest copyWith(
          void Function(DisconnectMemberRequest) updates) =>
      super.copyWith((message) => updates(message as DisconnectMemberRequest))
          as DisconnectMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DisconnectMemberRequest create() => DisconnectMemberRequest._();
  @$core.override
  DisconnectMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DisconnectMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DisconnectMemberRequest>(create);
  static DisconnectMemberRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get slug => $_getSZ(0);
  @$pb.TagNumber(1)
  set slug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlug() => $_clearField(1);
}

class DisconnectMemberResponse extends $pb.GeneratedMessage {
  factory DisconnectMemberResponse() => create();

  DisconnectMemberResponse._();

  factory DisconnectMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DisconnectMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DisconnectMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DisconnectMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DisconnectMemberResponse copyWith(
          void Function(DisconnectMemberResponse) updates) =>
      super.copyWith((message) => updates(message as DisconnectMemberResponse))
          as DisconnectMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DisconnectMemberResponse create() => DisconnectMemberResponse._();
  @$core.override
  DisconnectMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DisconnectMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DisconnectMemberResponse>(create);
  static DisconnectMemberResponse? _defaultInstance;
}

class TellRequest extends $pb.GeneratedMessage {
  factory TellRequest({
    $core.String? memberSlug,
    $core.String? method,
    $core.List<$core.int>? payload,
  }) {
    final result = create();
    if (memberSlug != null) result.memberSlug = memberSlug;
    if (method != null) result.method = method;
    if (payload != null) result.payload = payload;
    return result;
  }

  TellRequest._();

  factory TellRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TellRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TellRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'memberSlug')
    ..aOS(2, _omitFieldNames ? '' : 'method')
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'payload', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TellRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TellRequest copyWith(void Function(TellRequest) updates) =>
      super.copyWith((message) => updates(message as TellRequest))
          as TellRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TellRequest create() => TellRequest._();
  @$core.override
  TellRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TellRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TellRequest>(create);
  static TellRequest? _defaultInstance;

  /// Which member to address (by slug).
  @$pb.TagNumber(1)
  $core.String get memberSlug => $_getSZ(0);
  @$pb.TagNumber(1)
  set memberSlug($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMemberSlug() => $_has(0);
  @$pb.TagNumber(1)
  void clearMemberSlug() => $_clearField(1);

  /// Fully qualified RPC method name
  /// (e.g. "greeting.v1.GreetingService/SayHello").
  @$pb.TagNumber(2)
  $core.String get method => $_getSZ(1);
  @$pb.TagNumber(2)
  set method($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMethod() => $_has(1);
  @$pb.TagNumber(2)
  void clearMethod() => $_clearField(2);

  /// JSON-encoded request payload.
  /// The organism deserializes and forwards as protobuf to the member.
  @$pb.TagNumber(3)
  $core.List<$core.int> get payload => $_getN(2);
  @$pb.TagNumber(3)
  set payload($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPayload() => $_has(2);
  @$pb.TagNumber(3)
  void clearPayload() => $_clearField(3);
}

class TellResponse extends $pb.GeneratedMessage {
  factory TellResponse({
    $core.List<$core.int>? payload,
  }) {
    final result = create();
    if (payload != null) result.payload = payload;
    return result;
  }

  TellResponse._();

  factory TellResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TellResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TellResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..a<$core.List<$core.int>>(
        1, _omitFieldNames ? '' : 'payload', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TellResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TellResponse copyWith(void Function(TellResponse) updates) =>
      super.copyWith((message) => updates(message as TellResponse))
          as TellResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TellResponse create() => TellResponse._();
  @$core.override
  TellResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TellResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TellResponse>(create);
  static TellResponse? _defaultInstance;

  /// JSON-encoded response from the member.
  @$pb.TagNumber(1)
  $core.List<$core.int> get payload => $_getN(0);
  @$pb.TagNumber(1)
  set payload($core.List<$core.int> value) => $_setBytes(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPayload() => $_has(0);
  @$pb.TagNumber(1)
  void clearPayload() => $_clearField(1);
}

class TurnOffCoaxRequest extends $pb.GeneratedMessage {
  factory TurnOffCoaxRequest() => create();

  TurnOffCoaxRequest._();

  factory TurnOffCoaxRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TurnOffCoaxRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnOffCoaxRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnOffCoaxRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnOffCoaxRequest copyWith(void Function(TurnOffCoaxRequest) updates) =>
      super.copyWith((message) => updates(message as TurnOffCoaxRequest))
          as TurnOffCoaxRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TurnOffCoaxRequest create() => TurnOffCoaxRequest._();
  @$core.override
  TurnOffCoaxRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TurnOffCoaxRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TurnOffCoaxRequest>(create);
  static TurnOffCoaxRequest? _defaultInstance;
}

class TurnOffCoaxResponse extends $pb.GeneratedMessage {
  factory TurnOffCoaxResponse() => create();

  TurnOffCoaxResponse._();

  factory TurnOffCoaxResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TurnOffCoaxResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnOffCoaxResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'holons.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnOffCoaxResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnOffCoaxResponse copyWith(void Function(TurnOffCoaxResponse) updates) =>
      super.copyWith((message) => updates(message as TurnOffCoaxResponse))
          as TurnOffCoaxResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TurnOffCoaxResponse create() => TurnOffCoaxResponse._();
  @$core.override
  TurnOffCoaxResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TurnOffCoaxResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TurnOffCoaxResponse>(create);
  static TurnOffCoaxResponse? _defaultInstance;
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
