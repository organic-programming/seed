// This is a generated file - do not edit.
//
// Generated from relay/v1/relay.proto.

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

@$core.Deprecated('Use tickRequestDescriptor instead')
const TickRequest$json = {
  '1': 'TickRequest',
  '2': [
    {'1': 'sender', '3': 1, '4': 1, '5': 9, '10': 'sender'},
    {'1': 'note', '3': 2, '4': 1, '5': 9, '10': 'note'},
  ],
};

/// Descriptor for `TickRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tickRequestDescriptor = $convert.base64Decode(
    'CgtUaWNrUmVxdWVzdBIWCgZzZW5kZXIYASABKAlSBnNlbmRlchISCgRub3RlGAIgASgJUgRub3'
    'Rl');

@$core.Deprecated('Use hopReceiptDescriptor instead')
const HopReceipt$json = {
  '1': 'HopReceipt',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
    {'1': 'uid', '3': 2, '4': 1, '5': 9, '10': 'uid'},
    {'1': 'received', '3': 3, '4': 1, '5': 3, '10': 'received'},
  ],
};

/// Descriptor for `HopReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hopReceiptDescriptor = $convert.base64Decode(
    'CgpIb3BSZWNlaXB0EhIKBHNsdWcYASABKAlSBHNsdWcSEAoDdWlkGAIgASgJUgN1aWQSGgoIcm'
    'VjZWl2ZWQYAyABKANSCHJlY2VpdmVk');

@$core.Deprecated('Use tickResponseDescriptor instead')
const TickResponse$json = {
  '1': 'TickResponse',
  '2': [
    {'1': 'responder_slug', '3': 1, '4': 1, '5': 9, '10': 'responderSlug'},
    {
      '1': 'responder_instance_uid',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'responderInstanceUid'
    },
    {
      '1': 'hops',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.relay.v1.HopReceipt',
      '10': 'hops'
    },
  ],
};

/// Descriptor for `TickResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tickResponseDescriptor = $convert.base64Decode(
    'CgxUaWNrUmVzcG9uc2USJQoOcmVzcG9uZGVyX3NsdWcYASABKAlSDXJlc3BvbmRlclNsdWcSNA'
    'oWcmVzcG9uZGVyX2luc3RhbmNlX3VpZBgCIAEoCVIUcmVzcG9uZGVySW5zdGFuY2VVaWQSKAoE'
    'aG9wcxgDIAMoCzIULnJlbGF5LnYxLkhvcFJlY2VpcHRSBGhvcHM=');
