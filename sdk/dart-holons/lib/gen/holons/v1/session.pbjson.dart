// This is a generated file - do not edit.
//
// Generated from holons/v1/session.proto.

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

@$core.Deprecated('Use sessionEventKindDescriptor instead')
const SessionEventKind$json = {
  '1': 'SessionEventKind',
  '2': [
    {'1': 'SESSION_EVENT_KIND_UNSPECIFIED', '2': 0},
    {'1': 'SNAPSHOT', '2': 1},
    {'1': 'SESSION_CREATED', '2': 2},
    {'1': 'STATE_CHANGED', '2': 3},
    {'1': 'METRICS_UPDATED', '2': 4},
    {'1': 'SESSION_CLOSED', '2': 5},
  ],
};

/// Descriptor for `SessionEventKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sessionEventKindDescriptor = $convert.base64Decode(
    'ChBTZXNzaW9uRXZlbnRLaW5kEiIKHlNFU1NJT05fRVZFTlRfS0lORF9VTlNQRUNJRklFRBAAEg'
    'wKCFNOQVBTSE9UEAESEwoPU0VTU0lPTl9DUkVBVEVEEAISEQoNU1RBVEVfQ0hBTkdFRBADEhMK'
    'D01FVFJJQ1NfVVBEQVRFRBAEEhIKDlNFU1NJT05fQ0xPU0VEEAU=');

@$core.Deprecated('Use sessionStateDescriptor instead')
const SessionState$json = {
  '1': 'SessionState',
  '2': [
    {'1': 'SESSION_STATE_UNSPECIFIED', '2': 0},
    {'1': 'CONNECTING', '2': 1},
    {'1': 'ACTIVE', '2': 2},
    {'1': 'STALE', '2': 3},
    {'1': 'DRAINING', '2': 4},
    {'1': 'FAILED', '2': 5},
    {'1': 'CLOSED', '2': 6},
  ],
};

/// Descriptor for `SessionState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sessionStateDescriptor = $convert.base64Decode(
    'CgxTZXNzaW9uU3RhdGUSHQoZU0VTU0lPTl9TVEFURV9VTlNQRUNJRklFRBAAEg4KCkNPTk5FQ1'
    'RJTkcQARIKCgZBQ1RJVkUQAhIJCgVTVEFMRRADEgwKCERSQUlOSU5HEAQSCgoGRkFJTEVEEAUS'
    'CgoGQ0xPU0VEEAY=');

@$core.Deprecated('Use sessionDirectionDescriptor instead')
const SessionDirection$json = {
  '1': 'SessionDirection',
  '2': [
    {'1': 'SESSION_DIRECTION_UNSPECIFIED', '2': 0},
    {'1': 'INBOUND', '2': 1},
    {'1': 'OUTBOUND', '2': 2},
  ],
};

/// Descriptor for `SessionDirection`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sessionDirectionDescriptor = $convert.base64Decode(
    'ChBTZXNzaW9uRGlyZWN0aW9uEiEKHVNFU1NJT05fRElSRUNUSU9OX1VOU1BFQ0lGSUVEEAASCw'
    'oHSU5CT1VORBABEgwKCE9VVEJPVU5EEAI=');

@$core.Deprecated('Use sessionsRequestDescriptor instead')
const SessionsRequest$json = {
  '1': 'SessionsRequest',
  '2': [
    {
      '1': 'state_filter',
      '3': 1,
      '4': 3,
      '5': 14,
      '6': '.holons.v1.SessionState',
      '10': 'stateFilter'
    },
    {
      '1': 'direction_filter',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.SessionDirection',
      '10': 'directionFilter'
    },
    {'1': 'include_closed', '3': 3, '4': 1, '5': 8, '10': 'includeClosed'},
    {'1': 'limit', '3': 4, '4': 1, '5': 5, '10': 'limit'},
    {'1': 'page_token', '3': 5, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `SessionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sessionsRequestDescriptor = $convert.base64Decode(
    'Cg9TZXNzaW9uc1JlcXVlc3QSOgoMc3RhdGVfZmlsdGVyGAEgAygOMhcuaG9sb25zLnYxLlNlc3'
    'Npb25TdGF0ZVILc3RhdGVGaWx0ZXISRgoQZGlyZWN0aW9uX2ZpbHRlchgCIAEoDjIbLmhvbG9u'
    'cy52MS5TZXNzaW9uRGlyZWN0aW9uUg9kaXJlY3Rpb25GaWx0ZXISJQoOaW5jbHVkZV9jbG9zZW'
    'QYAyABKAhSDWluY2x1ZGVDbG9zZWQSFAoFbGltaXQYBCABKAVSBWxpbWl0Eh0KCnBhZ2VfdG9r'
    'ZW4YBSABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use sessionsResponseDescriptor instead')
const SessionsResponse$json = {
  '1': 'SessionsResponse',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
    {
      '1': 'sessions',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.SessionInfo',
      '10': 'sessions'
    },
    {'1': 'next_page_token', '3': 3, '4': 1, '5': 9, '10': 'nextPageToken'},
    {'1': 'total_count', '3': 4, '4': 1, '5': 5, '10': 'totalCount'},
  ],
};

/// Descriptor for `SessionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sessionsResponseDescriptor = $convert.base64Decode(
    'ChBTZXNzaW9uc1Jlc3BvbnNlEhIKBHNsdWcYASABKAlSBHNsdWcSMgoIc2Vzc2lvbnMYAiADKA'
    'syFi5ob2xvbnMudjEuU2Vzc2lvbkluZm9SCHNlc3Npb25zEiYKD25leHRfcGFnZV90b2tlbhgD'
    'IAEoCVINbmV4dFBhZ2VUb2tlbhIfCgt0b3RhbF9jb3VudBgEIAEoBVIKdG90YWxDb3VudA==');

@$core.Deprecated('Use sessionInfoDescriptor instead')
const SessionInfo$json = {
  '1': 'SessionInfo',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'remote_slug', '3': 2, '4': 1, '5': 9, '10': 'remoteSlug'},
    {'1': 'transport', '3': 3, '4': 1, '5': 9, '10': 'transport'},
    {'1': 'address', '3': 4, '4': 1, '5': 9, '10': 'address'},
    {
      '1': 'direction',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.SessionDirection',
      '10': 'direction'
    },
    {
      '1': 'state',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.SessionState',
      '10': 'state'
    },
    {
      '1': 'started_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'startedAt'
    },
    {
      '1': 'state_changed_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'stateChangedAt'
    },
    {
      '1': 'ended_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'endedAt'
    },
    {'1': 'rpc_count', '3': 10, '4': 1, '5': 3, '10': 'rpcCount'},
    {
      '1': 'last_rpc_at',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'lastRpcAt'
    },
    {
      '1': 'metrics',
      '3': 20,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.SessionMetrics',
      '10': 'metrics'
    },
    {'1': 'mesh_host', '3': 21, '4': 1, '5': 9, '10': 'meshHost'},
    {'1': 'instance_uid', '3': 22, '4': 1, '5': 9, '10': 'instanceUid'},
  ],
};

/// Descriptor for `SessionInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sessionInfoDescriptor = $convert.base64Decode(
    'CgtTZXNzaW9uSW5mbxIdCgpzZXNzaW9uX2lkGAEgASgJUglzZXNzaW9uSWQSHwoLcmVtb3RlX3'
    'NsdWcYAiABKAlSCnJlbW90ZVNsdWcSHAoJdHJhbnNwb3J0GAMgASgJUgl0cmFuc3BvcnQSGAoH'
    'YWRkcmVzcxgEIAEoCVIHYWRkcmVzcxI5CglkaXJlY3Rpb24YBSABKA4yGy5ob2xvbnMudjEuU2'
    'Vzc2lvbkRpcmVjdGlvblIJZGlyZWN0aW9uEi0KBXN0YXRlGAYgASgOMhcuaG9sb25zLnYxLlNl'
    'c3Npb25TdGF0ZVIFc3RhdGUSOQoKc3RhcnRlZF9hdBgHIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi'
    '5UaW1lc3RhbXBSCXN0YXJ0ZWRBdBJEChBzdGF0ZV9jaGFuZ2VkX2F0GAggASgLMhouZ29vZ2xl'
    'LnByb3RvYnVmLlRpbWVzdGFtcFIOc3RhdGVDaGFuZ2VkQXQSNQoIZW5kZWRfYXQYCSABKAsyGi'
    '5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgdlbmRlZEF0EhsKCXJwY19jb3VudBgKIAEoA1II'
    'cnBjQ291bnQSOgoLbGFzdF9ycGNfYXQYCyABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW'
    '1wUglsYXN0UnBjQXQSMwoHbWV0cmljcxgUIAEoCzIZLmhvbG9ucy52MS5TZXNzaW9uTWV0cmlj'
    'c1IHbWV0cmljcxIbCgltZXNoX2hvc3QYFSABKAlSCG1lc2hIb3N0EiEKDGluc3RhbmNlX3VpZB'
    'gWIAEoCVILaW5zdGFuY2VVaWQ=');

@$core.Deprecated('Use sessionMetricsDescriptor instead')
const SessionMetrics$json = {
  '1': 'SessionMetrics',
  '2': [
    {'1': 'total_p50_us', '3': 1, '4': 1, '5': 3, '10': 'totalP50Us'},
    {'1': 'total_p99_us', '3': 2, '4': 1, '5': 3, '10': 'totalP99Us'},
    {'1': 'wire_out_p50_us', '3': 3, '4': 1, '5': 3, '10': 'wireOutP50Us'},
    {'1': 'wire_out_p99_us', '3': 4, '4': 1, '5': 3, '10': 'wireOutP99Us'},
    {'1': 'queue_p50_us', '3': 5, '4': 1, '5': 3, '10': 'queueP50Us'},
    {'1': 'queue_p99_us', '3': 6, '4': 1, '5': 3, '10': 'queueP99Us'},
    {'1': 'work_p50_us', '3': 7, '4': 1, '5': 3, '10': 'workP50Us'},
    {'1': 'work_p99_us', '3': 8, '4': 1, '5': 3, '10': 'workP99Us'},
    {'1': 'wire_in_p50_us', '3': 9, '4': 1, '5': 3, '10': 'wireInP50Us'},
    {'1': 'wire_in_p99_us', '3': 10, '4': 1, '5': 3, '10': 'wireInP99Us'},
    {'1': 'error_count', '3': 11, '4': 1, '5': 3, '10': 'errorCount'},
    {'1': 'bytes_sent', '3': 12, '4': 1, '5': 3, '10': 'bytesSent'},
    {'1': 'bytes_received', '3': 13, '4': 1, '5': 3, '10': 'bytesReceived'},
    {'1': 'in_flight', '3': 14, '4': 1, '5': 5, '10': 'inFlight'},
    {'1': 'messages_sent', '3': 15, '4': 1, '5': 3, '10': 'messagesSent'},
    {
      '1': 'messages_received',
      '3': 16,
      '4': 1,
      '5': 3,
      '10': 'messagesReceived'
    },
    {
      '1': 'methods',
      '3': 20,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.SessionMetrics.MethodsEntry',
      '10': 'methods'
    },
  ],
  '3': [SessionMetrics_MethodsEntry$json],
};

@$core.Deprecated('Use sessionMetricsDescriptor instead')
const SessionMetrics_MethodsEntry$json = {
  '1': 'MethodsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.MethodMetrics',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

/// Descriptor for `SessionMetrics`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sessionMetricsDescriptor = $convert.base64Decode(
    'Cg5TZXNzaW9uTWV0cmljcxIgCgx0b3RhbF9wNTBfdXMYASABKANSCnRvdGFsUDUwVXMSIAoMdG'
    '90YWxfcDk5X3VzGAIgASgDUgp0b3RhbFA5OVVzEiUKD3dpcmVfb3V0X3A1MF91cxgDIAEoA1IM'
    'd2lyZU91dFA1MFVzEiUKD3dpcmVfb3V0X3A5OV91cxgEIAEoA1IMd2lyZU91dFA5OVVzEiAKDH'
    'F1ZXVlX3A1MF91cxgFIAEoA1IKcXVldWVQNTBVcxIgCgxxdWV1ZV9wOTlfdXMYBiABKANSCnF1'
    'ZXVlUDk5VXMSHgoLd29ya19wNTBfdXMYByABKANSCXdvcmtQNTBVcxIeCgt3b3JrX3A5OV91cx'
    'gIIAEoA1IJd29ya1A5OVVzEiMKDndpcmVfaW5fcDUwX3VzGAkgASgDUgt3aXJlSW5QNTBVcxIj'
    'Cg53aXJlX2luX3A5OV91cxgKIAEoA1ILd2lyZUluUDk5VXMSHwoLZXJyb3JfY291bnQYCyABKA'
    'NSCmVycm9yQ291bnQSHQoKYnl0ZXNfc2VudBgMIAEoA1IJYnl0ZXNTZW50EiUKDmJ5dGVzX3Jl'
    'Y2VpdmVkGA0gASgDUg1ieXRlc1JlY2VpdmVkEhsKCWluX2ZsaWdodBgOIAEoBVIIaW5GbGlnaH'
    'QSIwoNbWVzc2FnZXNfc2VudBgPIAEoA1IMbWVzc2FnZXNTZW50EisKEW1lc3NhZ2VzX3JlY2Vp'
    'dmVkGBAgASgDUhBtZXNzYWdlc1JlY2VpdmVkEkAKB21ldGhvZHMYFCADKAsyJi5ob2xvbnMudj'
    'EuU2Vzc2lvbk1ldHJpY3MuTWV0aG9kc0VudHJ5UgdtZXRob2RzGlQKDE1ldGhvZHNFbnRyeRIQ'
    'CgNrZXkYASABKAlSA2tleRIuCgV2YWx1ZRgCIAEoCzIYLmhvbG9ucy52MS5NZXRob2RNZXRyaW'
    'NzUgV2YWx1ZToCOAE=');

@$core.Deprecated('Use methodMetricsDescriptor instead')
const MethodMetrics$json = {
  '1': 'MethodMetrics',
  '2': [
    {'1': 'call_count', '3': 1, '4': 1, '5': 3, '10': 'callCount'},
    {'1': 'error_count', '3': 2, '4': 1, '5': 3, '10': 'errorCount'},
    {'1': 'in_flight', '3': 3, '4': 1, '5': 5, '10': 'inFlight'},
    {'1': 'total_p50_us', '3': 10, '4': 1, '5': 3, '10': 'totalP50Us'},
    {'1': 'total_p99_us', '3': 11, '4': 1, '5': 3, '10': 'totalP99Us'},
    {'1': 'wire_out_p50_us', '3': 12, '4': 1, '5': 3, '10': 'wireOutP50Us'},
    {'1': 'wire_out_p99_us', '3': 13, '4': 1, '5': 3, '10': 'wireOutP99Us'},
    {'1': 'queue_p50_us', '3': 14, '4': 1, '5': 3, '10': 'queueP50Us'},
    {'1': 'queue_p99_us', '3': 15, '4': 1, '5': 3, '10': 'queueP99Us'},
    {'1': 'work_p50_us', '3': 16, '4': 1, '5': 3, '10': 'workP50Us'},
    {'1': 'work_p99_us', '3': 17, '4': 1, '5': 3, '10': 'workP99Us'},
    {'1': 'wire_in_p50_us', '3': 18, '4': 1, '5': 3, '10': 'wireInP50Us'},
    {'1': 'wire_in_p99_us', '3': 19, '4': 1, '5': 3, '10': 'wireInP99Us'},
  ],
};

/// Descriptor for `MethodMetrics`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List methodMetricsDescriptor = $convert.base64Decode(
    'Cg1NZXRob2RNZXRyaWNzEh0KCmNhbGxfY291bnQYASABKANSCWNhbGxDb3VudBIfCgtlcnJvcl'
    '9jb3VudBgCIAEoA1IKZXJyb3JDb3VudBIbCglpbl9mbGlnaHQYAyABKAVSCGluRmxpZ2h0EiAK'
    'DHRvdGFsX3A1MF91cxgKIAEoA1IKdG90YWxQNTBVcxIgCgx0b3RhbF9wOTlfdXMYCyABKANSCn'
    'RvdGFsUDk5VXMSJQoPd2lyZV9vdXRfcDUwX3VzGAwgASgDUgx3aXJlT3V0UDUwVXMSJQoPd2ly'
    'ZV9vdXRfcDk5X3VzGA0gASgDUgx3aXJlT3V0UDk5VXMSIAoMcXVldWVfcDUwX3VzGA4gASgDUg'
    'pxdWV1ZVA1MFVzEiAKDHF1ZXVlX3A5OV91cxgPIAEoA1IKcXVldWVQOTlVcxIeCgt3b3JrX3A1'
    'MF91cxgQIAEoA1IJd29ya1A1MFVzEh4KC3dvcmtfcDk5X3VzGBEgASgDUgl3b3JrUDk5VXMSIw'
    'oOd2lyZV9pbl9wNTBfdXMYEiABKANSC3dpcmVJblA1MFVzEiMKDndpcmVfaW5fcDk5X3VzGBMg'
    'ASgDUgt3aXJlSW5QOTlVcw==');

@$core.Deprecated('Use watchSessionsRequestDescriptor instead')
const WatchSessionsRequest$json = {
  '1': 'WatchSessionsRequest',
  '2': [
    {
      '1': 'state_filter',
      '3': 1,
      '4': 3,
      '5': 14,
      '6': '.holons.v1.SessionState',
      '10': 'stateFilter'
    },
    {
      '1': 'direction_filter',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.SessionDirection',
      '10': 'directionFilter'
    },
    {
      '1': 'send_initial_snapshot',
      '3': 3,
      '4': 1,
      '5': 8,
      '10': 'sendInitialSnapshot'
    },
  ],
};

/// Descriptor for `WatchSessionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchSessionsRequestDescriptor = $convert.base64Decode(
    'ChRXYXRjaFNlc3Npb25zUmVxdWVzdBI6CgxzdGF0ZV9maWx0ZXIYASADKA4yFy5ob2xvbnMudj'
    'EuU2Vzc2lvblN0YXRlUgtzdGF0ZUZpbHRlchJGChBkaXJlY3Rpb25fZmlsdGVyGAIgASgOMhsu'
    'aG9sb25zLnYxLlNlc3Npb25EaXJlY3Rpb25SD2RpcmVjdGlvbkZpbHRlchIyChVzZW5kX2luaX'
    'RpYWxfc25hcHNob3QYAyABKAhSE3NlbmRJbml0aWFsU25hcHNob3Q=');

@$core.Deprecated('Use sessionEventDescriptor instead')
const SessionEvent$json = {
  '1': 'SessionEvent',
  '2': [
    {
      '1': 'ts',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'ts'
    },
    {
      '1': 'kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.SessionEventKind',
      '10': 'kind'
    },
    {
      '1': 'session',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.SessionInfo',
      '10': 'session'
    },
    {
      '1': 'previous_state',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.SessionState',
      '10': 'previousState'
    },
  ],
};

/// Descriptor for `SessionEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sessionEventDescriptor = $convert.base64Decode(
    'CgxTZXNzaW9uRXZlbnQSKgoCdHMYASABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUg'
    'J0cxIvCgRraW5kGAIgASgOMhsuaG9sb25zLnYxLlNlc3Npb25FdmVudEtpbmRSBGtpbmQSMAoH'
    'c2Vzc2lvbhgDIAEoCzIWLmhvbG9ucy52MS5TZXNzaW9uSW5mb1IHc2Vzc2lvbhI+Cg5wcmV2aW'
    '91c19zdGF0ZRgEIAEoDjIXLmhvbG9ucy52MS5TZXNzaW9uU3RhdGVSDXByZXZpb3VzU3RhdGU=');
