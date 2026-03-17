// This is a generated file - do not edit.
//
// Generated from greeting/v1/greeting.proto.

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

@$core.Deprecated('Use listLanguagesRequestDescriptor instead')
const ListLanguagesRequest$json = {
  '1': 'ListLanguagesRequest',
};

/// Descriptor for `ListLanguagesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listLanguagesRequestDescriptor =
    $convert.base64Decode('ChRMaXN0TGFuZ3VhZ2VzUmVxdWVzdA==');

@$core.Deprecated('Use listLanguagesResponseDescriptor instead')
const ListLanguagesResponse$json = {
  '1': 'ListLanguagesResponse',
  '2': [
    {
      '1': 'languages',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.greeting.v1.Language',
      '10': 'languages'
    },
  ],
};

/// Descriptor for `ListLanguagesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listLanguagesResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0TGFuZ3VhZ2VzUmVzcG9uc2USMwoJbGFuZ3VhZ2VzGAEgAygLMhUuZ3JlZXRpbmcudj'
    'EuTGFuZ3VhZ2VSCWxhbmd1YWdlcw==');

@$core.Deprecated('Use languageDescriptor instead')
const Language$json = {
  '1': 'Language',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'native', '3': 3, '4': 1, '5': 9, '10': 'native'},
  ],
};

/// Descriptor for `Language`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List languageDescriptor = $convert.base64Decode(
    'CghMYW5ndWFnZRISCgRjb2RlGAEgASgJUgRjb2RlEhIKBG5hbWUYAiABKAlSBG5hbWUSFgoGbm'
    'F0aXZlGAMgASgJUgZuYXRpdmU=');

@$core.Deprecated('Use sayHelloRequestDescriptor instead')
const SayHelloRequest$json = {
  '1': 'SayHelloRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'lang_code', '3': 2, '4': 1, '5': 9, '10': 'langCode'},
  ],
};

/// Descriptor for `SayHelloRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sayHelloRequestDescriptor = $convert.base64Decode(
    'Cg9TYXlIZWxsb1JlcXVlc3QSEgoEbmFtZRgBIAEoCVIEbmFtZRIbCglsYW5nX2NvZGUYAiABKA'
    'lSCGxhbmdDb2Rl');

@$core.Deprecated('Use sayHelloResponseDescriptor instead')
const SayHelloResponse$json = {
  '1': 'SayHelloResponse',
  '2': [
    {'1': 'greeting', '3': 1, '4': 1, '5': 9, '10': 'greeting'},
    {'1': 'language', '3': 2, '4': 1, '5': 9, '10': 'language'},
    {'1': 'lang_code', '3': 3, '4': 1, '5': 9, '10': 'langCode'},
  ],
};

/// Descriptor for `SayHelloResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sayHelloResponseDescriptor = $convert.base64Decode(
    'ChBTYXlIZWxsb1Jlc3BvbnNlEhoKCGdyZWV0aW5nGAEgASgJUghncmVldGluZxIaCghsYW5ndW'
    'FnZRgCIAEoCVIIbGFuZ3VhZ2USGwoJbGFuZ19jb2RlGAMgASgJUghsYW5nQ29kZQ==');
