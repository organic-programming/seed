// This is a generated file - do not edit.
//
// Generated from holons/v1/coax.proto.

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

@$core.Deprecated('Use memberStateDescriptor instead')
const MemberState$json = {
  '1': 'MemberState',
  '2': [
    {'1': 'MEMBER_STATE_UNSPECIFIED', '2': 0},
    {'1': 'MEMBER_STATE_AVAILABLE', '2': 1},
    {'1': 'MEMBER_STATE_CONNECTING', '2': 2},
    {'1': 'MEMBER_STATE_CONNECTED', '2': 3},
    {'1': 'MEMBER_STATE_ERROR', '2': 4},
  ],
};

/// Descriptor for `MemberState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List memberStateDescriptor = $convert.base64Decode(
    'CgtNZW1iZXJTdGF0ZRIcChhNRU1CRVJfU1RBVEVfVU5TUEVDSUZJRUQQABIaChZNRU1CRVJfU1'
    'RBVEVfQVZBSUxBQkxFEAESGwoXTUVNQkVSX1NUQVRFX0NPTk5FQ1RJTkcQAhIaChZNRU1CRVJf'
    'U1RBVEVfQ09OTkVDVEVEEAMSFgoSTUVNQkVSX1NUQVRFX0VSUk9SEAQ=');

@$core.Deprecated('Use listMembersRequestDescriptor instead')
const ListMembersRequest$json = {
  '1': 'ListMembersRequest',
};

/// Descriptor for `ListMembersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMembersRequestDescriptor =
    $convert.base64Decode('ChJMaXN0TWVtYmVyc1JlcXVlc3Q=');

@$core.Deprecated('Use listMembersResponseDescriptor instead')
const ListMembersResponse$json = {
  '1': 'ListMembersResponse',
  '2': [
    {
      '1': 'members',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.MemberInfo',
      '10': 'members'
    },
  ],
};

/// Descriptor for `ListMembersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMembersResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0TWVtYmVyc1Jlc3BvbnNlEi8KB21lbWJlcnMYASADKAsyFS5ob2xvbnMudjEuTWVtYm'
    'VySW5mb1IHbWVtYmVycw==');

@$core.Deprecated('Use memberInfoDescriptor instead')
const MemberInfo$json = {
  '1': 'MemberInfo',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
    {
      '1': 'identity',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Identity',
      '10': 'identity'
    },
    {
      '1': 'state',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.MemberState',
      '10': 'state'
    },
    {'1': 'is_organism', '3': 4, '4': 1, '5': 8, '10': 'isOrganism'},
  ],
};

/// Descriptor for `MemberInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberInfoDescriptor = $convert.base64Decode(
    'CgpNZW1iZXJJbmZvEhIKBHNsdWcYASABKAlSBHNsdWcSPQoIaWRlbnRpdHkYAiABKAsyIS5ob2'
    'xvbnMudjEuSG9sb25NYW5pZmVzdC5JZGVudGl0eVIIaWRlbnRpdHkSLAoFc3RhdGUYAyABKA4y'
    'Fi5ob2xvbnMudjEuTWVtYmVyU3RhdGVSBXN0YXRlEh8KC2lzX29yZ2FuaXNtGAQgASgIUgppc0'
    '9yZ2FuaXNt');

@$core.Deprecated('Use memberStatusRequestDescriptor instead')
const MemberStatusRequest$json = {
  '1': 'MemberStatusRequest',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
  ],
};

/// Descriptor for `MemberStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberStatusRequestDescriptor = $convert
    .base64Decode('ChNNZW1iZXJTdGF0dXNSZXF1ZXN0EhIKBHNsdWcYASABKAlSBHNsdWc=');

@$core.Deprecated('Use memberStatusResponseDescriptor instead')
const MemberStatusResponse$json = {
  '1': 'MemberStatusResponse',
  '2': [
    {
      '1': 'member',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.MemberInfo',
      '10': 'member'
    },
  ],
};

/// Descriptor for `MemberStatusResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberStatusResponseDescriptor = $convert.base64Decode(
    'ChRNZW1iZXJTdGF0dXNSZXNwb25zZRItCgZtZW1iZXIYASABKAsyFS5ob2xvbnMudjEuTWVtYm'
    'VySW5mb1IGbWVtYmVy');

@$core.Deprecated('Use connectMemberRequestDescriptor instead')
const ConnectMemberRequest$json = {
  '1': 'ConnectMemberRequest',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
    {'1': 'transport', '3': 2, '4': 1, '5': 9, '10': 'transport'},
  ],
};

/// Descriptor for `ConnectMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List connectMemberRequestDescriptor = $convert.base64Decode(
    'ChRDb25uZWN0TWVtYmVyUmVxdWVzdBISCgRzbHVnGAEgASgJUgRzbHVnEhwKCXRyYW5zcG9ydB'
    'gCIAEoCVIJdHJhbnNwb3J0');

@$core.Deprecated('Use connectMemberResponseDescriptor instead')
const ConnectMemberResponse$json = {
  '1': 'ConnectMemberResponse',
  '2': [
    {
      '1': 'member',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.MemberInfo',
      '10': 'member'
    },
  ],
};

/// Descriptor for `ConnectMemberResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List connectMemberResponseDescriptor = $convert.base64Decode(
    'ChVDb25uZWN0TWVtYmVyUmVzcG9uc2USLQoGbWVtYmVyGAEgASgLMhUuaG9sb25zLnYxLk1lbW'
    'JlckluZm9SBm1lbWJlcg==');

@$core.Deprecated('Use disconnectMemberRequestDescriptor instead')
const DisconnectMemberRequest$json = {
  '1': 'DisconnectMemberRequest',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
  ],
};

/// Descriptor for `DisconnectMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List disconnectMemberRequestDescriptor =
    $convert.base64Decode(
        'ChdEaXNjb25uZWN0TWVtYmVyUmVxdWVzdBISCgRzbHVnGAEgASgJUgRzbHVn');

@$core.Deprecated('Use disconnectMemberResponseDescriptor instead')
const DisconnectMemberResponse$json = {
  '1': 'DisconnectMemberResponse',
};

/// Descriptor for `DisconnectMemberResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List disconnectMemberResponseDescriptor =
    $convert.base64Decode('ChhEaXNjb25uZWN0TWVtYmVyUmVzcG9uc2U=');

@$core.Deprecated('Use tellRequestDescriptor instead')
const TellRequest$json = {
  '1': 'TellRequest',
  '2': [
    {'1': 'member_slug', '3': 1, '4': 1, '5': 9, '10': 'memberSlug'},
    {'1': 'method', '3': 2, '4': 1, '5': 9, '10': 'method'},
    {'1': 'payload', '3': 3, '4': 1, '5': 12, '10': 'payload'},
  ],
};

/// Descriptor for `TellRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tellRequestDescriptor = $convert.base64Decode(
    'CgtUZWxsUmVxdWVzdBIfCgttZW1iZXJfc2x1ZxgBIAEoCVIKbWVtYmVyU2x1ZxIWCgZtZXRob2'
    'QYAiABKAlSBm1ldGhvZBIYCgdwYXlsb2FkGAMgASgMUgdwYXlsb2Fk');

@$core.Deprecated('Use tellResponseDescriptor instead')
const TellResponse$json = {
  '1': 'TellResponse',
  '2': [
    {'1': 'payload', '3': 1, '4': 1, '5': 12, '10': 'payload'},
  ],
};

/// Descriptor for `TellResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tellResponseDescriptor = $convert
    .base64Decode('CgxUZWxsUmVzcG9uc2USGAoHcGF5bG9hZBgBIAEoDFIHcGF5bG9hZA==');

@$core.Deprecated('Use turnOffCoaxRequestDescriptor instead')
const TurnOffCoaxRequest$json = {
  '1': 'TurnOffCoaxRequest',
};

/// Descriptor for `TurnOffCoaxRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnOffCoaxRequestDescriptor =
    $convert.base64Decode('ChJUdXJuT2ZmQ29heFJlcXVlc3Q=');

@$core.Deprecated('Use turnOffCoaxResponseDescriptor instead')
const TurnOffCoaxResponse$json = {
  '1': 'TurnOffCoaxResponse',
};

/// Descriptor for `TurnOffCoaxResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnOffCoaxResponseDescriptor =
    $convert.base64Decode('ChNUdXJuT2ZmQ29heFJlc3BvbnNl');
