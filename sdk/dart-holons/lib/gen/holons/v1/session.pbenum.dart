// This is a generated file - do not edit.
//
// Generated from holons/v1/session.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class SessionEventKind extends $pb.ProtobufEnum {
  static const SessionEventKind SESSION_EVENT_KIND_UNSPECIFIED =
      SessionEventKind._(
          0, _omitEnumNames ? '' : 'SESSION_EVENT_KIND_UNSPECIFIED');
  static const SessionEventKind SNAPSHOT =
      SessionEventKind._(1, _omitEnumNames ? '' : 'SNAPSHOT');
  static const SessionEventKind SESSION_CREATED =
      SessionEventKind._(2, _omitEnumNames ? '' : 'SESSION_CREATED');
  static const SessionEventKind STATE_CHANGED =
      SessionEventKind._(3, _omitEnumNames ? '' : 'STATE_CHANGED');
  static const SessionEventKind METRICS_UPDATED =
      SessionEventKind._(4, _omitEnumNames ? '' : 'METRICS_UPDATED');
  static const SessionEventKind SESSION_CLOSED =
      SessionEventKind._(5, _omitEnumNames ? '' : 'SESSION_CLOSED');

  static const $core.List<SessionEventKind> values = <SessionEventKind>[
    SESSION_EVENT_KIND_UNSPECIFIED,
    SNAPSHOT,
    SESSION_CREATED,
    STATE_CHANGED,
    METRICS_UPDATED,
    SESSION_CLOSED,
  ];

  static final $core.List<SessionEventKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static SessionEventKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SessionEventKind._(super.value, super.name);
}

class SessionState extends $pb.ProtobufEnum {
  static const SessionState SESSION_STATE_UNSPECIFIED =
      SessionState._(0, _omitEnumNames ? '' : 'SESSION_STATE_UNSPECIFIED');
  static const SessionState CONNECTING =
      SessionState._(1, _omitEnumNames ? '' : 'CONNECTING');
  static const SessionState ACTIVE =
      SessionState._(2, _omitEnumNames ? '' : 'ACTIVE');
  static const SessionState STALE =
      SessionState._(3, _omitEnumNames ? '' : 'STALE');
  static const SessionState DRAINING =
      SessionState._(4, _omitEnumNames ? '' : 'DRAINING');
  static const SessionState FAILED =
      SessionState._(5, _omitEnumNames ? '' : 'FAILED');
  static const SessionState CLOSED =
      SessionState._(6, _omitEnumNames ? '' : 'CLOSED');

  static const $core.List<SessionState> values = <SessionState>[
    SESSION_STATE_UNSPECIFIED,
    CONNECTING,
    ACTIVE,
    STALE,
    DRAINING,
    FAILED,
    CLOSED,
  ];

  static final $core.List<SessionState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static SessionState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SessionState._(super.value, super.name);
}

class SessionDirection extends $pb.ProtobufEnum {
  static const SessionDirection SESSION_DIRECTION_UNSPECIFIED =
      SessionDirection._(
          0, _omitEnumNames ? '' : 'SESSION_DIRECTION_UNSPECIFIED');
  static const SessionDirection INBOUND =
      SessionDirection._(1, _omitEnumNames ? '' : 'INBOUND');
  static const SessionDirection OUTBOUND =
      SessionDirection._(2, _omitEnumNames ? '' : 'OUTBOUND');

  static const $core.List<SessionDirection> values = <SessionDirection>[
    SESSION_DIRECTION_UNSPECIFIED,
    INBOUND,
    OUTBOUND,
  ];

  static final $core.List<SessionDirection?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SessionDirection? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SessionDirection._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
