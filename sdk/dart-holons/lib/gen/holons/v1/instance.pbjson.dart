// This is a generated file - do not edit.
//
// Generated from holons/v1/instance.proto.

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

@$core.Deprecated('Use listInstancesRequestDescriptor instead')
const ListInstancesRequest$json = {
  '1': 'ListInstancesRequest',
  '2': [
    {'1': 'slugs', '3': 1, '4': 3, '5': 9, '10': 'slugs'},
    {'1': 'include_stale', '3': 2, '4': 1, '5': 8, '10': 'includeStale'},
  ],
};

/// Descriptor for `ListInstancesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listInstancesRequestDescriptor = $convert.base64Decode(
    'ChRMaXN0SW5zdGFuY2VzUmVxdWVzdBIUCgVzbHVncxgBIAMoCVIFc2x1Z3MSIwoNaW5jbHVkZV'
    '9zdGFsZRgCIAEoCFIMaW5jbHVkZVN0YWxl');

@$core.Deprecated('Use listInstancesResponseDescriptor instead')
const ListInstancesResponse$json = {
  '1': 'ListInstancesResponse',
  '2': [
    {
      '1': 'instances',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.InstanceInfo',
      '10': 'instances'
    },
  ],
};

/// Descriptor for `ListInstancesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listInstancesResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0SW5zdGFuY2VzUmVzcG9uc2USNQoJaW5zdGFuY2VzGAEgAygLMhcuaG9sb25zLnYxLk'
    'luc3RhbmNlSW5mb1IJaW5zdGFuY2Vz');

@$core.Deprecated('Use getInstanceRequestDescriptor instead')
const GetInstanceRequest$json = {
  '1': 'GetInstanceRequest',
  '2': [
    {'1': 'uid', '3': 1, '4': 1, '5': 9, '10': 'uid'},
  ],
};

/// Descriptor for `GetInstanceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getInstanceRequestDescriptor = $convert
    .base64Decode('ChJHZXRJbnN0YW5jZVJlcXVlc3QSEAoDdWlkGAEgASgJUgN1aWQ=');

@$core.Deprecated('Use instanceInfoDescriptor instead')
const InstanceInfo$json = {
  '1': 'InstanceInfo',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
    {'1': 'uid', '3': 2, '4': 1, '5': 9, '10': 'uid'},
    {'1': 'pid', '3': 3, '4': 1, '5': 5, '10': 'pid'},
    {
      '1': 'started_at',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'startedAt'
    },
    {'1': 'mode', '3': 5, '4': 1, '5': 9, '10': 'mode'},
    {'1': 'transport', '3': 6, '4': 1, '5': 9, '10': 'transport'},
    {'1': 'address', '3': 7, '4': 1, '5': 9, '10': 'address'},
    {'1': 'metrics_addr', '3': 8, '4': 1, '5': 9, '10': 'metricsAddr'},
    {'1': 'log_path', '3': 9, '4': 1, '5': 9, '10': 'logPath'},
    {'1': 'default', '3': 10, '4': 1, '5': 8, '10': 'default'},
    {'1': 'stale', '3': 11, '4': 1, '5': 8, '10': 'stale'},
    {'1': 'organism_uid', '3': 12, '4': 1, '5': 9, '10': 'organismUid'},
    {'1': 'organism_slug', '3': 13, '4': 1, '5': 9, '10': 'organismSlug'},
  ],
};

/// Descriptor for `InstanceInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List instanceInfoDescriptor = $convert.base64Decode(
    'CgxJbnN0YW5jZUluZm8SEgoEc2x1ZxgBIAEoCVIEc2x1ZxIQCgN1aWQYAiABKAlSA3VpZBIQCg'
    'NwaWQYAyABKAVSA3BpZBI5CgpzdGFydGVkX2F0GAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRp'
    'bWVzdGFtcFIJc3RhcnRlZEF0EhIKBG1vZGUYBSABKAlSBG1vZGUSHAoJdHJhbnNwb3J0GAYgAS'
    'gJUgl0cmFuc3BvcnQSGAoHYWRkcmVzcxgHIAEoCVIHYWRkcmVzcxIhCgxtZXRyaWNzX2FkZHIY'
    'CCABKAlSC21ldHJpY3NBZGRyEhkKCGxvZ19wYXRoGAkgASgJUgdsb2dQYXRoEhgKB2RlZmF1bH'
    'QYCiABKAhSB2RlZmF1bHQSFAoFc3RhbGUYCyABKAhSBXN0YWxlEiEKDG9yZ2FuaXNtX3VpZBgM'
    'IAEoCVILb3JnYW5pc21VaWQSIwoNb3JnYW5pc21fc2x1ZxgNIAEoCVIMb3JnYW5pc21TbHVn');
