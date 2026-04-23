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

/// ObservabilityVisibility gates the HolonSession and HolonObservability
/// services. Declared at file level so protos that need to reference it
/// outside the manifest (manifest extensions, tooling) can import it.
class ObservabilityVisibility extends $pb.ProtobufEnum {
  static const ObservabilityVisibility OBSERVABILITY_VISIBILITY_UNSPECIFIED =
      ObservabilityVisibility._(
          0, _omitEnumNames ? '' : 'OBSERVABILITY_VISIBILITY_UNSPECIFIED');
  static const ObservabilityVisibility OBSERVABILITY_VISIBILITY_OFF =
      ObservabilityVisibility._(
          1, _omitEnumNames ? '' : 'OBSERVABILITY_VISIBILITY_OFF');
  static const ObservabilityVisibility OBSERVABILITY_VISIBILITY_SUMMARY =
      ObservabilityVisibility._(
          2, _omitEnumNames ? '' : 'OBSERVABILITY_VISIBILITY_SUMMARY');
  static const ObservabilityVisibility OBSERVABILITY_VISIBILITY_FULL =
      ObservabilityVisibility._(
          3, _omitEnumNames ? '' : 'OBSERVABILITY_VISIBILITY_FULL');

  static const $core.List<ObservabilityVisibility> values =
      <ObservabilityVisibility>[
    OBSERVABILITY_VISIBILITY_UNSPECIFIED,
    OBSERVABILITY_VISIBILITY_OFF,
    OBSERVABILITY_VISIBILITY_SUMMARY,
    OBSERVABILITY_VISIBILITY_FULL,
  ];

  static final $core.List<ObservabilityVisibility?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ObservabilityVisibility? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ObservabilityVisibility._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
