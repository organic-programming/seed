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

/// Structurally mirrors opentelemetry.proto.logs.v1.SeverityNumber.
class SeverityNumber extends $pb.ProtobufEnum {
  static const SeverityNumber SEVERITY_NUMBER_UNSPECIFIED =
      SeverityNumber._(0, _omitEnumNames ? '' : 'SEVERITY_NUMBER_UNSPECIFIED');
  static const SeverityNumber SEVERITY_NUMBER_TRACE =
      SeverityNumber._(1, _omitEnumNames ? '' : 'SEVERITY_NUMBER_TRACE');
  static const SeverityNumber SEVERITY_NUMBER_DEBUG =
      SeverityNumber._(5, _omitEnumNames ? '' : 'SEVERITY_NUMBER_DEBUG');
  static const SeverityNumber SEVERITY_NUMBER_INFO =
      SeverityNumber._(9, _omitEnumNames ? '' : 'SEVERITY_NUMBER_INFO');
  static const SeverityNumber SEVERITY_NUMBER_WARN =
      SeverityNumber._(13, _omitEnumNames ? '' : 'SEVERITY_NUMBER_WARN');
  static const SeverityNumber SEVERITY_NUMBER_ERROR =
      SeverityNumber._(17, _omitEnumNames ? '' : 'SEVERITY_NUMBER_ERROR');
  static const SeverityNumber SEVERITY_NUMBER_FATAL =
      SeverityNumber._(21, _omitEnumNames ? '' : 'SEVERITY_NUMBER_FATAL');

  static const $core.List<SeverityNumber> values = <SeverityNumber>[
    SEVERITY_NUMBER_UNSPECIFIED,
    SEVERITY_NUMBER_TRACE,
    SEVERITY_NUMBER_DEBUG,
    SEVERITY_NUMBER_INFO,
    SEVERITY_NUMBER_WARN,
    SEVERITY_NUMBER_ERROR,
    SEVERITY_NUMBER_FATAL,
  ];

  static final $core.Map<$core.int, SeverityNumber> _byValue =
      $pb.ProtobufEnum.initByValue(values);
  static SeverityNumber? valueOf($core.int value) => _byValue[value];

  const SeverityNumber._(super.value, super.name);
}

class AggregationTemporality extends $pb.ProtobufEnum {
  static const AggregationTemporality AGGREGATION_TEMPORALITY_UNSPECIFIED =
      AggregationTemporality._(
          0, _omitEnumNames ? '' : 'AGGREGATION_TEMPORALITY_UNSPECIFIED');
  static const AggregationTemporality AGGREGATION_TEMPORALITY_DELTA =
      AggregationTemporality._(
          1, _omitEnumNames ? '' : 'AGGREGATION_TEMPORALITY_DELTA');
  static const AggregationTemporality AGGREGATION_TEMPORALITY_CUMULATIVE =
      AggregationTemporality._(
          2, _omitEnumNames ? '' : 'AGGREGATION_TEMPORALITY_CUMULATIVE');

  static const $core.List<AggregationTemporality> values =
      <AggregationTemporality>[
    AGGREGATION_TEMPORALITY_UNSPECIFIED,
    AGGREGATION_TEMPORALITY_DELTA,
    AGGREGATION_TEMPORALITY_CUMULATIVE,
  ];

  static final $core.List<AggregationTemporality?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static AggregationTemporality? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AggregationTemporality._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
