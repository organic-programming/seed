// This is a generated file - do not edit.
//
// Generated from holons/v1/observability.proto.

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

@$core.Deprecated('Use logLevelDescriptor instead')
const LogLevel$json = {
  '1': 'LogLevel',
  '2': [
    {'1': 'LOG_LEVEL_UNSPECIFIED', '2': 0},
    {'1': 'TRACE', '2': 1},
    {'1': 'DEBUG', '2': 2},
    {'1': 'INFO', '2': 3},
    {'1': 'WARN', '2': 4},
    {'1': 'ERROR', '2': 5},
    {'1': 'FATAL', '2': 6},
  ],
};

/// Descriptor for `LogLevel`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List logLevelDescriptor = $convert.base64Decode(
    'CghMb2dMZXZlbBIZChVMT0dfTEVWRUxfVU5TUEVDSUZJRUQQABIJCgVUUkFDRRABEgkKBURFQl'
    'VHEAISCAoESU5GTxADEggKBFdBUk4QBBIJCgVFUlJPUhAFEgkKBUZBVEFMEAY=');

@$core.Deprecated('Use eventTypeDescriptor instead')
const EventType$json = {
  '1': 'EventType',
  '2': [
    {'1': 'EVENT_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'INSTANCE_SPAWNED', '2': 1},
    {'1': 'INSTANCE_READY', '2': 2},
    {'1': 'INSTANCE_EXITED', '2': 3},
    {'1': 'INSTANCE_CRASHED', '2': 4},
    {'1': 'SESSION_STARTED', '2': 5},
    {'1': 'SESSION_ENDED', '2': 6},
    {'1': 'HANDLER_PANIC', '2': 7},
    {'1': 'CONFIG_RELOADED', '2': 8},
  ],
};

/// Descriptor for `EventType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List eventTypeDescriptor = $convert.base64Decode(
    'CglFdmVudFR5cGUSGgoWRVZFTlRfVFlQRV9VTlNQRUNJRklFRBAAEhQKEElOU1RBTkNFX1NQQV'
    'dORUQQARISCg5JTlNUQU5DRV9SRUFEWRACEhMKD0lOU1RBTkNFX0VYSVRFRBADEhQKEElOU1RB'
    'TkNFX0NSQVNIRUQQBBITCg9TRVNTSU9OX1NUQVJURUQQBRIRCg1TRVNTSU9OX0VOREVEEAYSEQ'
    'oNSEFORExFUl9QQU5JQxAHEhMKD0NPTkZJR19SRUxPQURFRBAI');

@$core.Deprecated('Use logsRequestDescriptor instead')
const LogsRequest$json = {
  '1': 'LogsRequest',
  '2': [
    {
      '1': 'min_level',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.LogLevel',
      '10': 'minLevel'
    },
    {'1': 'session_ids', '3': 2, '4': 3, '5': 9, '10': 'sessionIds'},
    {'1': 'rpc_methods', '3': 3, '4': 3, '5': 9, '10': 'rpcMethods'},
    {
      '1': 'since',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Duration',
      '10': 'since'
    },
    {'1': 'follow', '3': 5, '4': 1, '5': 8, '10': 'follow'},
  ],
};

/// Descriptor for `LogsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List logsRequestDescriptor = $convert.base64Decode(
    'CgtMb2dzUmVxdWVzdBIwCgltaW5fbGV2ZWwYASABKA4yEy5ob2xvbnMudjEuTG9nTGV2ZWxSCG'
    '1pbkxldmVsEh8KC3Nlc3Npb25faWRzGAIgAygJUgpzZXNzaW9uSWRzEh8KC3JwY19tZXRob2Rz'
    'GAMgAygJUgpycGNNZXRob2RzEi8KBXNpbmNlGAQgASgLMhkuZ29vZ2xlLnByb3RvYnVmLkR1cm'
    'F0aW9uUgVzaW5jZRIWCgZmb2xsb3cYBSABKAhSBmZvbGxvdw==');

@$core.Deprecated('Use logEntryDescriptor instead')
const LogEntry$json = {
  '1': 'LogEntry',
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
      '1': 'level',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.LogLevel',
      '10': 'level'
    },
    {'1': 'slug', '3': 3, '4': 1, '5': 9, '10': 'slug'},
    {'1': 'instance_uid', '3': 4, '4': 1, '5': 9, '10': 'instanceUid'},
    {'1': 'session_id', '3': 5, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'rpc_method', '3': 6, '4': 1, '5': 9, '10': 'rpcMethod'},
    {'1': 'message', '3': 7, '4': 1, '5': 9, '10': 'message'},
    {
      '1': 'fields',
      '3': 8,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.LogEntry.FieldsEntry',
      '10': 'fields'
    },
    {'1': 'caller', '3': 9, '4': 1, '5': 9, '10': 'caller'},
    {
      '1': 'chain',
      '3': 10,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.ChainHop',
      '10': 'chain'
    },
  ],
  '3': [LogEntry_FieldsEntry$json],
};

@$core.Deprecated('Use logEntryDescriptor instead')
const LogEntry_FieldsEntry$json = {
  '1': 'FieldsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `LogEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List logEntryDescriptor = $convert.base64Decode(
    'CghMb2dFbnRyeRIqCgJ0cxgBIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSAnRzEi'
    'kKBWxldmVsGAIgASgOMhMuaG9sb25zLnYxLkxvZ0xldmVsUgVsZXZlbBISCgRzbHVnGAMgASgJ'
    'UgRzbHVnEiEKDGluc3RhbmNlX3VpZBgEIAEoCVILaW5zdGFuY2VVaWQSHQoKc2Vzc2lvbl9pZB'
    'gFIAEoCVIJc2Vzc2lvbklkEh0KCnJwY19tZXRob2QYBiABKAlSCXJwY01ldGhvZBIYCgdtZXNz'
    'YWdlGAcgASgJUgdtZXNzYWdlEjcKBmZpZWxkcxgIIAMoCzIfLmhvbG9ucy52MS5Mb2dFbnRyeS'
    '5GaWVsZHNFbnRyeVIGZmllbGRzEhYKBmNhbGxlchgJIAEoCVIGY2FsbGVyEikKBWNoYWluGAog'
    'AygLMhMuaG9sb25zLnYxLkNoYWluSG9wUgVjaGFpbho5CgtGaWVsZHNFbnRyeRIQCgNrZXkYAS'
    'ABKAlSA2tleRIUCgV2YWx1ZRgCIAEoCVIFdmFsdWU6AjgB');

@$core.Deprecated('Use chainHopDescriptor instead')
const ChainHop$json = {
  '1': 'ChainHop',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
    {'1': 'instance_uid', '3': 2, '4': 1, '5': 9, '10': 'instanceUid'},
  ],
};

/// Descriptor for `ChainHop`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chainHopDescriptor = $convert.base64Decode(
    'CghDaGFpbkhvcBISCgRzbHVnGAEgASgJUgRzbHVnEiEKDGluc3RhbmNlX3VpZBgCIAEoCVILaW'
    '5zdGFuY2VVaWQ=');

@$core.Deprecated('Use metricsRequestDescriptor instead')
const MetricsRequest$json = {
  '1': 'MetricsRequest',
  '2': [
    {'1': 'name_prefixes', '3': 1, '4': 3, '5': 9, '10': 'namePrefixes'},
    {
      '1': 'include_session_rollup',
      '3': 2,
      '4': 1,
      '5': 8,
      '10': 'includeSessionRollup'
    },
  ],
};

/// Descriptor for `MetricsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List metricsRequestDescriptor = $convert.base64Decode(
    'Cg5NZXRyaWNzUmVxdWVzdBIjCg1uYW1lX3ByZWZpeGVzGAEgAygJUgxuYW1lUHJlZml4ZXMSNA'
    'oWaW5jbHVkZV9zZXNzaW9uX3JvbGx1cBgCIAEoCFIUaW5jbHVkZVNlc3Npb25Sb2xsdXA=');

@$core.Deprecated('Use metricsSnapshotDescriptor instead')
const MetricsSnapshot$json = {
  '1': 'MetricsSnapshot',
  '2': [
    {
      '1': 'captured_at',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'capturedAt'
    },
    {'1': 'slug', '3': 2, '4': 1, '5': 9, '10': 'slug'},
    {'1': 'instance_uid', '3': 3, '4': 1, '5': 9, '10': 'instanceUid'},
    {
      '1': 'samples',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.MetricSample',
      '10': 'samples'
    },
    {
      '1': 'session_rollup',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.SessionMetrics',
      '10': 'sessionRollup'
    },
  ],
};

/// Descriptor for `MetricsSnapshot`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List metricsSnapshotDescriptor = $convert.base64Decode(
    'Cg9NZXRyaWNzU25hcHNob3QSOwoLY2FwdHVyZWRfYXQYASABKAsyGi5nb29nbGUucHJvdG9idW'
    'YuVGltZXN0YW1wUgpjYXB0dXJlZEF0EhIKBHNsdWcYAiABKAlSBHNsdWcSIQoMaW5zdGFuY2Vf'
    'dWlkGAMgASgJUgtpbnN0YW5jZVVpZBIxCgdzYW1wbGVzGAQgAygLMhcuaG9sb25zLnYxLk1ldH'
    'JpY1NhbXBsZVIHc2FtcGxlcxJACg5zZXNzaW9uX3JvbGx1cBgFIAEoCzIZLmhvbG9ucy52MS5T'
    'ZXNzaW9uTWV0cmljc1INc2Vzc2lvblJvbGx1cA==');

@$core.Deprecated('Use metricSampleDescriptor instead')
const MetricSample$json = {
  '1': 'MetricSample',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {
      '1': 'labels',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.MetricSample.LabelsEntry',
      '10': 'labels'
    },
    {'1': 'counter', '3': 3, '4': 1, '5': 3, '9': 0, '10': 'counter'},
    {'1': 'gauge', '3': 4, '4': 1, '5': 1, '9': 0, '10': 'gauge'},
    {
      '1': 'histogram',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HistogramSample',
      '9': 0,
      '10': 'histogram'
    },
    {'1': 'help', '3': 6, '4': 1, '5': 9, '10': 'help'},
    {
      '1': 'chain',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.ChainHop',
      '10': 'chain'
    },
  ],
  '3': [MetricSample_LabelsEntry$json],
  '8': [
    {'1': 'value'},
  ],
};

@$core.Deprecated('Use metricSampleDescriptor instead')
const MetricSample_LabelsEntry$json = {
  '1': 'LabelsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `MetricSample`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List metricSampleDescriptor = $convert.base64Decode(
    'CgxNZXRyaWNTYW1wbGUSEgoEbmFtZRgBIAEoCVIEbmFtZRI7CgZsYWJlbHMYAiADKAsyIy5ob2'
    'xvbnMudjEuTWV0cmljU2FtcGxlLkxhYmVsc0VudHJ5UgZsYWJlbHMSGgoHY291bnRlchgDIAEo'
    'A0gAUgdjb3VudGVyEhYKBWdhdWdlGAQgASgBSABSBWdhdWdlEjoKCWhpc3RvZ3JhbRgFIAEoCz'
    'IaLmhvbG9ucy52MS5IaXN0b2dyYW1TYW1wbGVIAFIJaGlzdG9ncmFtEhIKBGhlbHAYBiABKAlS'
    'BGhlbHASKQoFY2hhaW4YByADKAsyEy5ob2xvbnMudjEuQ2hhaW5Ib3BSBWNoYWluGjkKC0xhYm'
    'Vsc0VudHJ5EhAKA2tleRgBIAEoCVIDa2V5EhQKBXZhbHVlGAIgASgJUgV2YWx1ZToCOAFCBwoF'
    'dmFsdWU=');

@$core.Deprecated('Use histogramSampleDescriptor instead')
const HistogramSample$json = {
  '1': 'HistogramSample',
  '2': [
    {
      '1': 'buckets',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.Bucket',
      '10': 'buckets'
    },
    {'1': 'count', '3': 2, '4': 1, '5': 3, '10': 'count'},
    {'1': 'sum', '3': 3, '4': 1, '5': 1, '10': 'sum'},
  ],
};

/// Descriptor for `HistogramSample`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List histogramSampleDescriptor = $convert.base64Decode(
    'Cg9IaXN0b2dyYW1TYW1wbGUSKwoHYnVja2V0cxgBIAMoCzIRLmhvbG9ucy52MS5CdWNrZXRSB2'
    'J1Y2tldHMSFAoFY291bnQYAiABKANSBWNvdW50EhAKA3N1bRgDIAEoAVIDc3Vt');

@$core.Deprecated('Use bucketDescriptor instead')
const Bucket$json = {
  '1': 'Bucket',
  '2': [
    {'1': 'upper_bound', '3': 1, '4': 1, '5': 1, '10': 'upperBound'},
    {'1': 'count', '3': 2, '4': 1, '5': 3, '10': 'count'},
  ],
};

/// Descriptor for `Bucket`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bucketDescriptor = $convert.base64Decode(
    'CgZCdWNrZXQSHwoLdXBwZXJfYm91bmQYASABKAFSCnVwcGVyQm91bmQSFAoFY291bnQYAiABKA'
    'NSBWNvdW50');

@$core.Deprecated('Use eventsRequestDescriptor instead')
const EventsRequest$json = {
  '1': 'EventsRequest',
  '2': [
    {
      '1': 'types',
      '3': 1,
      '4': 3,
      '5': 14,
      '6': '.holons.v1.EventType',
      '10': 'types'
    },
    {
      '1': 'since',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Duration',
      '10': 'since'
    },
    {'1': 'follow', '3': 3, '4': 1, '5': 8, '10': 'follow'},
  ],
};

/// Descriptor for `EventsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List eventsRequestDescriptor = $convert.base64Decode(
    'Cg1FdmVudHNSZXF1ZXN0EioKBXR5cGVzGAEgAygOMhQuaG9sb25zLnYxLkV2ZW50VHlwZVIFdH'
    'lwZXMSLwoFc2luY2UYAiABKAsyGS5nb29nbGUucHJvdG9idWYuRHVyYXRpb25SBXNpbmNlEhYK'
    'BmZvbGxvdxgDIAEoCFIGZm9sbG93');

@$core.Deprecated('Use eventInfoDescriptor instead')
const EventInfo$json = {
  '1': 'EventInfo',
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
      '1': 'type',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.EventType',
      '10': 'type'
    },
    {'1': 'slug', '3': 3, '4': 1, '5': 9, '10': 'slug'},
    {'1': 'instance_uid', '3': 4, '4': 1, '5': 9, '10': 'instanceUid'},
    {'1': 'session_id', '3': 5, '4': 1, '5': 9, '10': 'sessionId'},
    {
      '1': 'payload',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.EventInfo.PayloadEntry',
      '10': 'payload'
    },
    {
      '1': 'chain',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.ChainHop',
      '10': 'chain'
    },
  ],
  '3': [EventInfo_PayloadEntry$json],
};

@$core.Deprecated('Use eventInfoDescriptor instead')
const EventInfo_PayloadEntry$json = {
  '1': 'PayloadEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `EventInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List eventInfoDescriptor = $convert.base64Decode(
    'CglFdmVudEluZm8SKgoCdHMYASABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgJ0cx'
    'IoCgR0eXBlGAIgASgOMhQuaG9sb25zLnYxLkV2ZW50VHlwZVIEdHlwZRISCgRzbHVnGAMgASgJ'
    'UgRzbHVnEiEKDGluc3RhbmNlX3VpZBgEIAEoCVILaW5zdGFuY2VVaWQSHQoKc2Vzc2lvbl9pZB'
    'gFIAEoCVIJc2Vzc2lvbklkEjsKB3BheWxvYWQYBiADKAsyIS5ob2xvbnMudjEuRXZlbnRJbmZv'
    'LlBheWxvYWRFbnRyeVIHcGF5bG9hZBIpCgVjaGFpbhgHIAMoCzITLmhvbG9ucy52MS5DaGFpbk'
    'hvcFIFY2hhaW4aOgoMUGF5bG9hZEVudHJ5EhAKA2tleRgBIAEoCVIDa2V5EhQKBXZhbHVlGAIg'
    'ASgJUgV2YWx1ZToCOAE=');
