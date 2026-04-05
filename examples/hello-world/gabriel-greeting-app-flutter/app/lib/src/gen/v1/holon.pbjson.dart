// This is a generated file - do not edit.
//
// Generated from v1/holon.proto.

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

@$core.Deprecated('Use selectHolonRequestDescriptor instead')
const SelectHolonRequest$json = {
  '1': 'SelectHolonRequest',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
  ],
};

/// Descriptor for `SelectHolonRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List selectHolonRequestDescriptor = $convert
    .base64Decode('ChJTZWxlY3RIb2xvblJlcXVlc3QSEgoEc2x1ZxgBIAEoCVIEc2x1Zw==');

@$core.Deprecated('Use selectHolonResponseDescriptor instead')
const SelectHolonResponse$json = {
  '1': 'SelectHolonResponse',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
    {'1': 'display_name', '3': 2, '4': 1, '5': 9, '10': 'displayName'},
  ],
};

/// Descriptor for `SelectHolonResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List selectHolonResponseDescriptor = $convert.base64Decode(
    'ChNTZWxlY3RIb2xvblJlc3BvbnNlEhIKBHNsdWcYASABKAlSBHNsdWcSIQoMZGlzcGxheV9uYW'
    '1lGAIgASgJUgtkaXNwbGF5TmFtZQ==');

@$core.Deprecated('Use selectLanguageRequestDescriptor instead')
const SelectLanguageRequest$json = {
  '1': 'SelectLanguageRequest',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
  ],
};

/// Descriptor for `SelectLanguageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List selectLanguageRequestDescriptor =
    $convert.base64Decode(
        'ChVTZWxlY3RMYW5ndWFnZVJlcXVlc3QSEgoEY29kZRgBIAEoCVIEY29kZQ==');

@$core.Deprecated('Use selectLanguageResponseDescriptor instead')
const SelectLanguageResponse$json = {
  '1': 'SelectLanguageResponse',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
  ],
};

/// Descriptor for `SelectLanguageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List selectLanguageResponseDescriptor =
    $convert.base64Decode(
        'ChZTZWxlY3RMYW5ndWFnZVJlc3BvbnNlEhIKBGNvZGUYASABKAlSBGNvZGU=');

@$core.Deprecated('Use greetRequestDescriptor instead')
const GreetRequest$json = {
  '1': 'GreetRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'lang_code', '3': 2, '4': 1, '5': 9, '10': 'langCode'},
  ],
};

/// Descriptor for `GreetRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List greetRequestDescriptor = $convert.base64Decode(
    'CgxHcmVldFJlcXVlc3QSEgoEbmFtZRgBIAEoCVIEbmFtZRIbCglsYW5nX2NvZGUYAiABKAlSCG'
    'xhbmdDb2Rl');

@$core.Deprecated('Use greetResponseDescriptor instead')
const GreetResponse$json = {
  '1': 'GreetResponse',
  '2': [
    {'1': 'greeting', '3': 1, '4': 1, '5': 9, '10': 'greeting'},
  ],
};

/// Descriptor for `GreetResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List greetResponseDescriptor = $convert.base64Decode(
    'Cg1HcmVldFJlc3BvbnNlEhoKCGdyZWV0aW5nGAEgASgJUghncmVldGluZw==');
