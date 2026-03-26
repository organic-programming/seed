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

/// FieldLabel indicates field cardinality.
class FieldLabel extends $pb.ProtobufEnum {
  static const FieldLabel FIELD_LABEL_UNSPECIFIED =
      FieldLabel._(0, _omitEnumNames ? '' : 'FIELD_LABEL_UNSPECIFIED');
  static const FieldLabel FIELD_LABEL_OPTIONAL =
      FieldLabel._(1, _omitEnumNames ? '' : 'FIELD_LABEL_OPTIONAL');
  static const FieldLabel FIELD_LABEL_REPEATED =
      FieldLabel._(2, _omitEnumNames ? '' : 'FIELD_LABEL_REPEATED');
  static const FieldLabel FIELD_LABEL_MAP =
      FieldLabel._(3, _omitEnumNames ? '' : 'FIELD_LABEL_MAP');
  static const FieldLabel FIELD_LABEL_REQUIRED =
      FieldLabel._(4, _omitEnumNames ? '' : 'FIELD_LABEL_REQUIRED');

  static const $core.List<FieldLabel> values = <FieldLabel>[
    FIELD_LABEL_UNSPECIFIED,
    FIELD_LABEL_OPTIONAL,
    FIELD_LABEL_REPEATED,
    FIELD_LABEL_MAP,
    FIELD_LABEL_REQUIRED,
  ];

  static final $core.List<FieldLabel?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static FieldLabel? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const FieldLabel._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
