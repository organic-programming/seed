// This is a generated file - do not edit.
//
// Generated from holons/v1/describe.proto.

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

@$core.Deprecated('Use fieldLabelDescriptor instead')
const FieldLabel$json = {
  '1': 'FieldLabel',
  '2': [
    {'1': 'FIELD_LABEL_UNSPECIFIED', '2': 0},
    {'1': 'FIELD_LABEL_OPTIONAL', '2': 1},
    {'1': 'FIELD_LABEL_REPEATED', '2': 2},
    {'1': 'FIELD_LABEL_MAP', '2': 3},
    {'1': 'FIELD_LABEL_REQUIRED', '2': 4},
  ],
};

/// Descriptor for `FieldLabel`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List fieldLabelDescriptor = $convert.base64Decode(
    'CgpGaWVsZExhYmVsEhsKF0ZJRUxEX0xBQkVMX1VOU1BFQ0lGSUVEEAASGAoURklFTERfTEFCRU'
    'xfT1BUSU9OQUwQARIYChRGSUVMRF9MQUJFTF9SRVBFQVRFRBACEhMKD0ZJRUxEX0xBQkVMX01B'
    'UBADEhgKFEZJRUxEX0xBQkVMX1JFUVVJUkVEEAQ=');

@$core.Deprecated('Use describeRequestDescriptor instead')
const DescribeRequest$json = {
  '1': 'DescribeRequest',
};

/// Descriptor for `DescribeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List describeRequestDescriptor =
    $convert.base64Decode('Cg9EZXNjcmliZVJlcXVlc3Q=');

@$core.Deprecated('Use describeResponseDescriptor instead')
const DescribeResponse$json = {
  '1': 'DescribeResponse',
  '2': [
    {
      '1': 'manifest',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest',
      '10': 'manifest'
    },
    {
      '1': 'services',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.ServiceDoc',
      '10': 'services'
    },
  ],
};

/// Descriptor for `DescribeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List describeResponseDescriptor = $convert.base64Decode(
    'ChBEZXNjcmliZVJlc3BvbnNlEjQKCG1hbmlmZXN0GAEgASgLMhguaG9sb25zLnYxLkhvbG9uTW'
    'FuaWZlc3RSCG1hbmlmZXN0EjEKCHNlcnZpY2VzGAIgAygLMhUuaG9sb25zLnYxLlNlcnZpY2VE'
    'b2NSCHNlcnZpY2Vz');

@$core.Deprecated('Use serviceDocDescriptor instead')
const ServiceDoc$json = {
  '1': 'ServiceDoc',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'methods',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.MethodDoc',
      '10': 'methods'
    },
  ],
};

/// Descriptor for `ServiceDoc`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List serviceDocDescriptor = $convert.base64Decode(
    'CgpTZXJ2aWNlRG9jEhIKBG5hbWUYASABKAlSBG5hbWUSIAoLZGVzY3JpcHRpb24YAiABKAlSC2'
    'Rlc2NyaXB0aW9uEi4KB21ldGhvZHMYAyADKAsyFC5ob2xvbnMudjEuTWV0aG9kRG9jUgdtZXRo'
    'b2Rz');

@$core.Deprecated('Use methodDocDescriptor instead')
const MethodDoc$json = {
  '1': 'MethodDoc',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {'1': 'input_type', '3': 3, '4': 1, '5': 9, '10': 'inputType'},
    {'1': 'output_type', '3': 4, '4': 1, '5': 9, '10': 'outputType'},
    {
      '1': 'input_fields',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.FieldDoc',
      '10': 'inputFields'
    },
    {
      '1': 'output_fields',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.FieldDoc',
      '10': 'outputFields'
    },
    {'1': 'client_streaming', '3': 7, '4': 1, '5': 8, '10': 'clientStreaming'},
    {'1': 'server_streaming', '3': 8, '4': 1, '5': 8, '10': 'serverStreaming'},
    {'1': 'example_input', '3': 9, '4': 1, '5': 9, '10': 'exampleInput'},
  ],
};

/// Descriptor for `MethodDoc`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List methodDocDescriptor = $convert.base64Decode(
    'CglNZXRob2REb2MSEgoEbmFtZRgBIAEoCVIEbmFtZRIgCgtkZXNjcmlwdGlvbhgCIAEoCVILZG'
    'VzY3JpcHRpb24SHQoKaW5wdXRfdHlwZRgDIAEoCVIJaW5wdXRUeXBlEh8KC291dHB1dF90eXBl'
    'GAQgASgJUgpvdXRwdXRUeXBlEjYKDGlucHV0X2ZpZWxkcxgFIAMoCzITLmhvbG9ucy52MS5GaW'
    'VsZERvY1ILaW5wdXRGaWVsZHMSOAoNb3V0cHV0X2ZpZWxkcxgGIAMoCzITLmhvbG9ucy52MS5G'
    'aWVsZERvY1IMb3V0cHV0RmllbGRzEikKEGNsaWVudF9zdHJlYW1pbmcYByABKAhSD2NsaWVudF'
    'N0cmVhbWluZxIpChBzZXJ2ZXJfc3RyZWFtaW5nGAggASgIUg9zZXJ2ZXJTdHJlYW1pbmcSIwoN'
    'ZXhhbXBsZV9pbnB1dBgJIAEoCVIMZXhhbXBsZUlucHV0');

@$core.Deprecated('Use fieldDocDescriptor instead')
const FieldDoc$json = {
  '1': 'FieldDoc',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'type', '3': 2, '4': 1, '5': 9, '10': 'type'},
    {'1': 'number', '3': 3, '4': 1, '5': 5, '10': 'number'},
    {'1': 'description', '3': 4, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'label',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.FieldLabel',
      '10': 'label'
    },
    {'1': 'map_key_type', '3': 6, '4': 1, '5': 9, '10': 'mapKeyType'},
    {'1': 'map_value_type', '3': 7, '4': 1, '5': 9, '10': 'mapValueType'},
    {
      '1': 'nested_fields',
      '3': 8,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.FieldDoc',
      '10': 'nestedFields'
    },
    {
      '1': 'enum_values',
      '3': 9,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.EnumValueDoc',
      '10': 'enumValues'
    },
    {'1': 'required', '3': 10, '4': 1, '5': 8, '10': 'required'},
    {'1': 'example', '3': 11, '4': 1, '5': 9, '10': 'example'},
  ],
};

/// Descriptor for `FieldDoc`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fieldDocDescriptor = $convert.base64Decode(
    'CghGaWVsZERvYxISCgRuYW1lGAEgASgJUgRuYW1lEhIKBHR5cGUYAiABKAlSBHR5cGUSFgoGbn'
    'VtYmVyGAMgASgFUgZudW1iZXISIAoLZGVzY3JpcHRpb24YBCABKAlSC2Rlc2NyaXB0aW9uEisK'
    'BWxhYmVsGAUgASgOMhUuaG9sb25zLnYxLkZpZWxkTGFiZWxSBWxhYmVsEiAKDG1hcF9rZXlfdH'
    'lwZRgGIAEoCVIKbWFwS2V5VHlwZRIkCg5tYXBfdmFsdWVfdHlwZRgHIAEoCVIMbWFwVmFsdWVU'
    'eXBlEjgKDW5lc3RlZF9maWVsZHMYCCADKAsyEy5ob2xvbnMudjEuRmllbGREb2NSDG5lc3RlZE'
    'ZpZWxkcxI4CgtlbnVtX3ZhbHVlcxgJIAMoCzIXLmhvbG9ucy52MS5FbnVtVmFsdWVEb2NSCmVu'
    'dW1WYWx1ZXMSGgoIcmVxdWlyZWQYCiABKAhSCHJlcXVpcmVkEhgKB2V4YW1wbGUYCyABKAlSB2'
    'V4YW1wbGU=');

@$core.Deprecated('Use enumValueDocDescriptor instead')
const EnumValueDoc$json = {
  '1': 'EnumValueDoc',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'number', '3': 2, '4': 1, '5': 5, '10': 'number'},
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
  ],
};

/// Descriptor for `EnumValueDoc`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List enumValueDocDescriptor = $convert.base64Decode(
    'CgxFbnVtVmFsdWVEb2MSEgoEbmFtZRgBIAEoCVIEbmFtZRIWCgZudW1iZXIYAiABKAVSBm51bW'
    'JlchIgCgtkZXNjcmlwdGlvbhgDIAEoCVILZGVzY3JpcHRpb24=');
