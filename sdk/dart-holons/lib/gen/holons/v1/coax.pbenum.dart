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

/// MemberState represents the runtime lifecycle of a member.
class MemberState extends $pb.ProtobufEnum {
  static const MemberState MEMBER_STATE_UNSPECIFIED =
      MemberState._(0, _omitEnumNames ? '' : 'MEMBER_STATE_UNSPECIFIED');

  /// Known but not running.
  static const MemberState MEMBER_STATE_AVAILABLE =
      MemberState._(1, _omitEnumNames ? '' : 'MEMBER_STATE_AVAILABLE');

  /// Process starting, not yet ready.
  static const MemberState MEMBER_STATE_CONNECTING =
      MemberState._(2, _omitEnumNames ? '' : 'MEMBER_STATE_CONNECTING');

  /// Connected and ready for RPC.
  static const MemberState MEMBER_STATE_CONNECTED =
      MemberState._(3, _omitEnumNames ? '' : 'MEMBER_STATE_CONNECTED');

  /// Connection or process error.
  static const MemberState MEMBER_STATE_ERROR =
      MemberState._(4, _omitEnumNames ? '' : 'MEMBER_STATE_ERROR');

  static const $core.List<MemberState> values = <MemberState>[
    MEMBER_STATE_UNSPECIFIED,
    MEMBER_STATE_AVAILABLE,
    MEMBER_STATE_CONNECTING,
    MEMBER_STATE_CONNECTED,
    MEMBER_STATE_ERROR,
  ];

  static final $core.List<MemberState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static MemberState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MemberState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
