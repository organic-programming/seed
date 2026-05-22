// This is a generated file - do not edit.
//
// Generated from observability_cascade/v1/service.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

@$core.Deprecated('Use runRequestDescriptor instead')
const RunRequest$json = {
  '1': 'RunRequest',
};

/// Descriptor for `RunRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runRequestDescriptor =
    $convert.base64Decode('CgpSdW5SZXF1ZXN0');

@$core.Deprecated('Use phaseResultDescriptor instead')
const PhaseResult$json = {
  '1': 'PhaseResult',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'pass', '3': 2, '4': 1, '5': 5, '10': 'pass'},
    {'1': 'fail', '3': 3, '4': 1, '5': 5, '10': 'fail'},
    {'1': 'failures', '3': 4, '4': 3, '5': 9, '10': 'failures'},
    {'1': 'elapsed_us', '3': 5, '4': 1, '5': 3, '10': 'elapsedUs'},
  ],
};

/// Descriptor for `PhaseResult`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List phaseResultDescriptor = $convert.base64Decode(
    'CgtQaGFzZVJlc3VsdBISCgRuYW1lGAEgASgJUgRuYW1lEhIKBHBhc3MYAiABKAVSBHBhc3MSEg'
    'oEZmFpbBgDIAEoBVIEZmFpbBIaCghmYWlsdXJlcxgEIAMoCVIIZmFpbHVyZXMSHQoKZWxhcHNl'
    'ZF91cxgFIAEoA1IJZWxhcHNlZFVz');

@$core.Deprecated('Use cascadeReportDescriptor instead')
const CascadeReport$json = {
  '1': 'CascadeReport',
  '2': [
    {'1': 'ticks', '3': 1, '4': 1, '5': 5, '10': 'ticks'},
    {'1': 'pass', '3': 2, '4': 1, '5': 5, '10': 'pass'},
    {'1': 'fail', '3': 3, '4': 1, '5': 5, '10': 'fail'},
    {
      '1': 'phases',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.observability_cascade.v1.PhaseResult',
      '10': 'phases'
    },
    {'1': 'name', '3': 5, '4': 1, '5': 9, '10': 'name'},
    {'1': 'elapsed_us', '3': 6, '4': 1, '5': 3, '10': 'elapsedUs'},
  ],
};

/// Descriptor for `CascadeReport`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cascadeReportDescriptor = $convert.base64Decode(
    'Cg1DYXNjYWRlUmVwb3J0EhQKBXRpY2tzGAEgASgFUgV0aWNrcxISCgRwYXNzGAIgASgFUgRwYX'
    'NzEhIKBGZhaWwYAyABKAVSBGZhaWwSPQoGcGhhc2VzGAQgAygLMiUub2JzZXJ2YWJpbGl0eV9j'
    'YXNjYWRlLnYxLlBoYXNlUmVzdWx0UgZwaGFzZXMSEgoEbmFtZRgFIAEoCVIEbmFtZRIdCgplbG'
    'Fwc2VkX3VzGAYgASgDUgllbGFwc2VkVXM=');

@$core.Deprecated('Use multiPatternReportDescriptor instead')
const MultiPatternReport$json = {
  '1': 'MultiPatternReport',
  '2': [
    {
      '1': 'patterns',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.observability_cascade.v1.CascadeReport',
      '10': 'patterns'
    },
    {'1': 'total_pass', '3': 2, '4': 1, '5': 5, '10': 'totalPass'},
    {'1': 'total_fail', '3': 3, '4': 1, '5': 5, '10': 'totalFail'},
    {'1': 'total_elapsed_us', '3': 4, '4': 1, '5': 3, '10': 'totalElapsedUs'},
  ],
};

/// Descriptor for `MultiPatternReport`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List multiPatternReportDescriptor = $convert.base64Decode(
    'ChJNdWx0aVBhdHRlcm5SZXBvcnQSQwoIcGF0dGVybnMYASADKAsyJy5vYnNlcnZhYmlsaXR5X2'
    'Nhc2NhZGUudjEuQ2FzY2FkZVJlcG9ydFIIcGF0dGVybnMSHQoKdG90YWxfcGFzcxgCIAEoBVIJ'
    'dG90YWxQYXNzEh0KCnRvdGFsX2ZhaWwYAyABKAVSCXRvdGFsRmFpbBIoChB0b3RhbF9lbGFwc2'
    'VkX3VzGAQgASgDUg50b3RhbEVsYXBzZWRVcw==');
