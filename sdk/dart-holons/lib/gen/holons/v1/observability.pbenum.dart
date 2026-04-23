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

import 'package:protobuf/protobuf.dart' as $pb;

class LogLevel extends $pb.ProtobufEnum {
  static const LogLevel LOG_LEVEL_UNSPECIFIED =
      LogLevel._(0, _omitEnumNames ? '' : 'LOG_LEVEL_UNSPECIFIED');
  static const LogLevel TRACE = LogLevel._(1, _omitEnumNames ? '' : 'TRACE');
  static const LogLevel DEBUG = LogLevel._(2, _omitEnumNames ? '' : 'DEBUG');
  static const LogLevel INFO = LogLevel._(3, _omitEnumNames ? '' : 'INFO');
  static const LogLevel WARN = LogLevel._(4, _omitEnumNames ? '' : 'WARN');
  static const LogLevel ERROR = LogLevel._(5, _omitEnumNames ? '' : 'ERROR');
  static const LogLevel FATAL = LogLevel._(6, _omitEnumNames ? '' : 'FATAL');

  static const $core.List<LogLevel> values = <LogLevel>[
    LOG_LEVEL_UNSPECIFIED,
    TRACE,
    DEBUG,
    INFO,
    WARN,
    ERROR,
    FATAL,
  ];

  static final $core.List<LogLevel?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static LogLevel? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const LogLevel._(super.value, super.name);
}

class EventType extends $pb.ProtobufEnum {
  static const EventType EVENT_TYPE_UNSPECIFIED =
      EventType._(0, _omitEnumNames ? '' : 'EVENT_TYPE_UNSPECIFIED');
  static const EventType INSTANCE_SPAWNED =
      EventType._(1, _omitEnumNames ? '' : 'INSTANCE_SPAWNED');
  static const EventType INSTANCE_READY =
      EventType._(2, _omitEnumNames ? '' : 'INSTANCE_READY');
  static const EventType INSTANCE_EXITED =
      EventType._(3, _omitEnumNames ? '' : 'INSTANCE_EXITED');
  static const EventType INSTANCE_CRASHED =
      EventType._(4, _omitEnumNames ? '' : 'INSTANCE_CRASHED');
  static const EventType SESSION_STARTED =
      EventType._(5, _omitEnumNames ? '' : 'SESSION_STARTED');
  static const EventType SESSION_ENDED =
      EventType._(6, _omitEnumNames ? '' : 'SESSION_ENDED');
  static const EventType HANDLER_PANIC =
      EventType._(7, _omitEnumNames ? '' : 'HANDLER_PANIC');
  static const EventType CONFIG_RELOADED =
      EventType._(8, _omitEnumNames ? '' : 'CONFIG_RELOADED');

  static const $core.List<EventType> values = <EventType>[
    EVENT_TYPE_UNSPECIFIED,
    INSTANCE_SPAWNED,
    INSTANCE_READY,
    INSTANCE_EXITED,
    INSTANCE_CRASHED,
    SESSION_STARTED,
    SESSION_ENDED,
    HANDLER_PANIC,
    CONFIG_RELOADED,
  ];

  static final $core.List<EventType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 8);
  static EventType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const EventType._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
