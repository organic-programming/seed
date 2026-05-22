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

@$core.Deprecated('Use severityNumberDescriptor instead')
const SeverityNumber$json = {
  '1': 'SeverityNumber',
  '2': [
    {'1': 'SEVERITY_NUMBER_UNSPECIFIED', '2': 0},
    {'1': 'SEVERITY_NUMBER_TRACE', '2': 1},
    {'1': 'SEVERITY_NUMBER_DEBUG', '2': 5},
    {'1': 'SEVERITY_NUMBER_INFO', '2': 9},
    {'1': 'SEVERITY_NUMBER_WARN', '2': 13},
    {'1': 'SEVERITY_NUMBER_ERROR', '2': 17},
    {'1': 'SEVERITY_NUMBER_FATAL', '2': 21},
  ],
};

/// Descriptor for `SeverityNumber`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List severityNumberDescriptor = $convert.base64Decode(
    'Cg5TZXZlcml0eU51bWJlchIfChtTRVZFUklUWV9OVU1CRVJfVU5TUEVDSUZJRUQQABIZChVTRV'
    'ZFUklUWV9OVU1CRVJfVFJBQ0UQARIZChVTRVZFUklUWV9OVU1CRVJfREVCVUcQBRIYChRTRVZF'
    'UklUWV9OVU1CRVJfSU5GTxAJEhgKFFNFVkVSSVRZX05VTUJFUl9XQVJOEA0SGQoVU0VWRVJJVF'
    'lfTlVNQkVSX0VSUk9SEBESGQoVU0VWRVJJVFlfTlVNQkVSX0ZBVEFMEBU=');

@$core.Deprecated('Use aggregationTemporalityDescriptor instead')
const AggregationTemporality$json = {
  '1': 'AggregationTemporality',
  '2': [
    {'1': 'AGGREGATION_TEMPORALITY_UNSPECIFIED', '2': 0},
    {'1': 'AGGREGATION_TEMPORALITY_DELTA', '2': 1},
    {'1': 'AGGREGATION_TEMPORALITY_CUMULATIVE', '2': 2},
  ],
};

/// Descriptor for `AggregationTemporality`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List aggregationTemporalityDescriptor = $convert.base64Decode(
    'ChZBZ2dyZWdhdGlvblRlbXBvcmFsaXR5EicKI0FHR1JFR0FUSU9OX1RFTVBPUkFMSVRZX1VOU1'
    'BFQ0lGSUVEEAASIQodQUdHUkVHQVRJT05fVEVNUE9SQUxJVFlfREVMVEEQARImCiJBR0dSRUdB'
    'VElPTl9URU1QT1JBTElUWV9DVU1VTEFUSVZFEAI=');

@$core.Deprecated('Use logsRequestDescriptor instead')
const LogsRequest$json = {
  '1': 'LogsRequest',
  '2': [
    {
      '1': 'min_severity_number',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.SeverityNumber',
      '10': 'minSeverityNumber'
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
    'CgtMb2dzUmVxdWVzdBJJChNtaW5fc2V2ZXJpdHlfbnVtYmVyGAEgASgOMhkuaG9sb25zLnYxLl'
    'NldmVyaXR5TnVtYmVyUhFtaW5TZXZlcml0eU51bWJlchIfCgtzZXNzaW9uX2lkcxgCIAMoCVIK'
    'c2Vzc2lvbklkcxIfCgtycGNfbWV0aG9kcxgDIAMoCVIKcnBjTWV0aG9kcxIvCgVzaW5jZRgEIA'
    'EoCzIZLmdvb2dsZS5wcm90b2J1Zi5EdXJhdGlvblIFc2luY2USFgoGZm9sbG93GAUgASgIUgZm'
    'b2xsb3c=');

@$core.Deprecated('Use metricsRequestDescriptor instead')
const MetricsRequest$json = {
  '1': 'MetricsRequest',
  '2': [
    {'1': 'name_prefixes', '3': 1, '4': 3, '5': 9, '10': 'namePrefixes'},
  ],
};

/// Descriptor for `MetricsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List metricsRequestDescriptor = $convert.base64Decode(
    'Cg5NZXRyaWNzUmVxdWVzdBIjCg1uYW1lX3ByZWZpeGVzGAEgAygJUgxuYW1lUHJlZml4ZXM=');

@$core.Deprecated('Use eventsRequestDescriptor instead')
const EventsRequest$json = {
  '1': 'EventsRequest',
  '2': [
    {'1': 'event_names', '3': 1, '4': 3, '5': 9, '10': 'eventNames'},
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
    'Cg1FdmVudHNSZXF1ZXN0Eh8KC2V2ZW50X25hbWVzGAEgAygJUgpldmVudE5hbWVzEi8KBXNpbm'
    'NlGAIgASgLMhkuZ29vZ2xlLnByb3RvYnVmLkR1cmF0aW9uUgVzaW5jZRIWCgZmb2xsb3cYAyAB'
    'KAhSBmZvbGxvdw==');

@$core.Deprecated('Use anyValueDescriptor instead')
const AnyValue$json = {
  '1': 'AnyValue',
  '2': [
    {'1': 'string_value', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'stringValue'},
    {'1': 'bool_value', '3': 2, '4': 1, '5': 8, '9': 0, '10': 'boolValue'},
    {'1': 'int_value', '3': 3, '4': 1, '5': 3, '9': 0, '10': 'intValue'},
    {'1': 'double_value', '3': 4, '4': 1, '5': 1, '9': 0, '10': 'doubleValue'},
  ],
  '8': [
    {'1': 'value'},
  ],
};

/// Descriptor for `AnyValue`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List anyValueDescriptor = $convert.base64Decode(
    'CghBbnlWYWx1ZRIjCgxzdHJpbmdfdmFsdWUYASABKAlIAFILc3RyaW5nVmFsdWUSHwoKYm9vbF'
    '92YWx1ZRgCIAEoCEgAUglib29sVmFsdWUSHQoJaW50X3ZhbHVlGAMgASgDSABSCGludFZhbHVl'
    'EiMKDGRvdWJsZV92YWx1ZRgEIAEoAUgAUgtkb3VibGVWYWx1ZUIHCgV2YWx1ZQ==');

@$core.Deprecated('Use keyValueDescriptor instead')
const KeyValue$json = {
  '1': 'KeyValue',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.AnyValue',
      '10': 'value'
    },
  ],
};

/// Descriptor for `KeyValue`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List keyValueDescriptor = $convert.base64Decode(
    'CghLZXlWYWx1ZRIQCgNrZXkYASABKAlSA2tleRIpCgV2YWx1ZRgCIAEoCzITLmhvbG9ucy52MS'
    '5BbnlWYWx1ZVIFdmFsdWU=');

@$core.Deprecated('Use resourceDescriptor instead')
const Resource$json = {
  '1': 'Resource',
  '2': [
    {
      '1': 'attributes',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.KeyValue',
      '10': 'attributes'
    },
    {
      '1': 'dropped_attributes_count',
      '3': 2,
      '4': 1,
      '5': 13,
      '10': 'droppedAttributesCount'
    },
  ],
};

/// Descriptor for `Resource`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resourceDescriptor = $convert.base64Decode(
    'CghSZXNvdXJjZRIzCgphdHRyaWJ1dGVzGAEgAygLMhMuaG9sb25zLnYxLktleVZhbHVlUgphdH'
    'RyaWJ1dGVzEjgKGGRyb3BwZWRfYXR0cmlidXRlc19jb3VudBgCIAEoDVIWZHJvcHBlZEF0dHJp'
    'YnV0ZXNDb3VudA==');

@$core.Deprecated('Use logRecordDescriptor instead')
const LogRecord$json = {
  '1': 'LogRecord',
  '2': [
    {'1': 'time_unix_nano', '3': 1, '4': 1, '5': 6, '10': 'timeUnixNano'},
    {
      '1': 'severity_number',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.SeverityNumber',
      '10': 'severityNumber'
    },
    {'1': 'severity_text', '3': 3, '4': 1, '5': 9, '10': 'severityText'},
    {
      '1': 'body',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.AnyValue',
      '10': 'body'
    },
    {
      '1': 'attributes',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.KeyValue',
      '10': 'attributes'
    },
    {
      '1': 'dropped_attributes_count',
      '3': 7,
      '4': 1,
      '5': 13,
      '10': 'droppedAttributesCount'
    },
    {'1': 'flags', '3': 8, '4': 1, '5': 13, '10': 'flags'},
    {'1': 'trace_id', '3': 9, '4': 1, '5': 12, '10': 'traceId'},
    {'1': 'span_id', '3': 10, '4': 1, '5': 12, '10': 'spanId'},
    {
      '1': 'observed_time_unix_nano',
      '3': 11,
      '4': 1,
      '5': 6,
      '10': 'observedTimeUnixNano'
    },
    {'1': 'event_name', '3': 20, '4': 1, '5': 9, '10': 'eventName'},
    {'1': 'chain', '3': 21, '4': 3, '5': 9, '10': 'chain'},
  ],
};

/// Descriptor for `LogRecord`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List logRecordDescriptor = $convert.base64Decode(
    'CglMb2dSZWNvcmQSJAoOdGltZV91bml4X25hbm8YASABKAZSDHRpbWVVbml4TmFubxJCCg9zZX'
    'Zlcml0eV9udW1iZXIYAiABKA4yGS5ob2xvbnMudjEuU2V2ZXJpdHlOdW1iZXJSDnNldmVyaXR5'
    'TnVtYmVyEiMKDXNldmVyaXR5X3RleHQYAyABKAlSDHNldmVyaXR5VGV4dBInCgRib2R5GAUgAS'
    'gLMhMuaG9sb25zLnYxLkFueVZhbHVlUgRib2R5EjMKCmF0dHJpYnV0ZXMYBiADKAsyEy5ob2xv'
    'bnMudjEuS2V5VmFsdWVSCmF0dHJpYnV0ZXMSOAoYZHJvcHBlZF9hdHRyaWJ1dGVzX2NvdW50GA'
    'cgASgNUhZkcm9wcGVkQXR0cmlidXRlc0NvdW50EhQKBWZsYWdzGAggASgNUgVmbGFncxIZCgh0'
    'cmFjZV9pZBgJIAEoDFIHdHJhY2VJZBIXCgdzcGFuX2lkGAogASgMUgZzcGFuSWQSNQoXb2JzZX'
    'J2ZWRfdGltZV91bml4X25hbm8YCyABKAZSFG9ic2VydmVkVGltZVVuaXhOYW5vEh0KCmV2ZW50'
    'X25hbWUYFCABKAlSCWV2ZW50TmFtZRIUCgVjaGFpbhgVIAMoCVIFY2hhaW4=');

@$core.Deprecated('Use metricDescriptor instead')
const Metric$json = {
  '1': 'Metric',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {'1': 'unit', '3': 3, '4': 1, '5': 9, '10': 'unit'},
    {
      '1': 'gauge',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.Gauge',
      '9': 0,
      '10': 'gauge'
    },
    {
      '1': 'sum',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.Sum',
      '9': 0,
      '10': 'sum'
    },
    {
      '1': 'histogram',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.Histogram',
      '9': 0,
      '10': 'histogram'
    },
  ],
  '8': [
    {'1': 'data'},
  ],
};

/// Descriptor for `Metric`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List metricDescriptor = $convert.base64Decode(
    'CgZNZXRyaWMSEgoEbmFtZRgBIAEoCVIEbmFtZRIgCgtkZXNjcmlwdGlvbhgCIAEoCVILZGVzY3'
    'JpcHRpb24SEgoEdW5pdBgDIAEoCVIEdW5pdBIoCgVnYXVnZRgFIAEoCzIQLmhvbG9ucy52MS5H'
    'YXVnZUgAUgVnYXVnZRIiCgNzdW0YByABKAsyDi5ob2xvbnMudjEuU3VtSABSA3N1bRI0CgloaX'
    'N0b2dyYW0YCSABKAsyFC5ob2xvbnMudjEuSGlzdG9ncmFtSABSCWhpc3RvZ3JhbUIGCgRkYXRh');

@$core.Deprecated('Use gaugeDescriptor instead')
const Gauge$json = {
  '1': 'Gauge',
  '2': [
    {
      '1': 'data_points',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.NumberDataPoint',
      '10': 'dataPoints'
    },
  ],
};

/// Descriptor for `Gauge`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gaugeDescriptor = $convert.base64Decode(
    'CgVHYXVnZRI7CgtkYXRhX3BvaW50cxgBIAMoCzIaLmhvbG9ucy52MS5OdW1iZXJEYXRhUG9pbn'
    'RSCmRhdGFQb2ludHM=');

@$core.Deprecated('Use sumDescriptor instead')
const Sum$json = {
  '1': 'Sum',
  '2': [
    {
      '1': 'data_points',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.NumberDataPoint',
      '10': 'dataPoints'
    },
    {
      '1': 'aggregation_temporality',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.AggregationTemporality',
      '10': 'aggregationTemporality'
    },
    {'1': 'is_monotonic', '3': 3, '4': 1, '5': 8, '10': 'isMonotonic'},
  ],
};

/// Descriptor for `Sum`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sumDescriptor = $convert.base64Decode(
    'CgNTdW0SOwoLZGF0YV9wb2ludHMYASADKAsyGi5ob2xvbnMudjEuTnVtYmVyRGF0YVBvaW50Ug'
    'pkYXRhUG9pbnRzEloKF2FnZ3JlZ2F0aW9uX3RlbXBvcmFsaXR5GAIgASgOMiEuaG9sb25zLnYx'
    'LkFnZ3JlZ2F0aW9uVGVtcG9yYWxpdHlSFmFnZ3JlZ2F0aW9uVGVtcG9yYWxpdHkSIQoMaXNfbW'
    '9ub3RvbmljGAMgASgIUgtpc01vbm90b25pYw==');

@$core.Deprecated('Use histogramDescriptor instead')
const Histogram$json = {
  '1': 'Histogram',
  '2': [
    {
      '1': 'data_points',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.HistogramDataPoint',
      '10': 'dataPoints'
    },
    {
      '1': 'aggregation_temporality',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.holons.v1.AggregationTemporality',
      '10': 'aggregationTemporality'
    },
  ],
};

/// Descriptor for `Histogram`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List histogramDescriptor = $convert.base64Decode(
    'CglIaXN0b2dyYW0SPgoLZGF0YV9wb2ludHMYASADKAsyHS5ob2xvbnMudjEuSGlzdG9ncmFtRG'
    'F0YVBvaW50UgpkYXRhUG9pbnRzEloKF2FnZ3JlZ2F0aW9uX3RlbXBvcmFsaXR5GAIgASgOMiEu'
    'aG9sb25zLnYxLkFnZ3JlZ2F0aW9uVGVtcG9yYWxpdHlSFmFnZ3JlZ2F0aW9uVGVtcG9yYWxpdH'
    'k=');

@$core.Deprecated('Use numberDataPointDescriptor instead')
const NumberDataPoint$json = {
  '1': 'NumberDataPoint',
  '2': [
    {
      '1': 'start_time_unix_nano',
      '3': 2,
      '4': 1,
      '5': 6,
      '10': 'startTimeUnixNano'
    },
    {'1': 'time_unix_nano', '3': 3, '4': 1, '5': 6, '10': 'timeUnixNano'},
    {'1': 'as_double', '3': 4, '4': 1, '5': 1, '9': 0, '10': 'asDouble'},
    {'1': 'as_int', '3': 6, '4': 1, '5': 3, '9': 0, '10': 'asInt'},
    {
      '1': 'attributes',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.KeyValue',
      '10': 'attributes'
    },
  ],
  '8': [
    {'1': 'value'},
  ],
};

/// Descriptor for `NumberDataPoint`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List numberDataPointDescriptor = $convert.base64Decode(
    'Cg9OdW1iZXJEYXRhUG9pbnQSLwoUc3RhcnRfdGltZV91bml4X25hbm8YAiABKAZSEXN0YXJ0VG'
    'ltZVVuaXhOYW5vEiQKDnRpbWVfdW5peF9uYW5vGAMgASgGUgx0aW1lVW5peE5hbm8SHQoJYXNf'
    'ZG91YmxlGAQgASgBSABSCGFzRG91YmxlEhcKBmFzX2ludBgGIAEoA0gAUgVhc0ludBIzCgphdH'
    'RyaWJ1dGVzGAcgAygLMhMuaG9sb25zLnYxLktleVZhbHVlUgphdHRyaWJ1dGVzQgcKBXZhbHVl');

@$core.Deprecated('Use histogramDataPointDescriptor instead')
const HistogramDataPoint$json = {
  '1': 'HistogramDataPoint',
  '2': [
    {
      '1': 'start_time_unix_nano',
      '3': 2,
      '4': 1,
      '5': 6,
      '10': 'startTimeUnixNano'
    },
    {'1': 'time_unix_nano', '3': 3, '4': 1, '5': 6, '10': 'timeUnixNano'},
    {'1': 'count', '3': 4, '4': 1, '5': 4, '10': 'count'},
    {'1': 'sum', '3': 5, '4': 1, '5': 1, '10': 'sum'},
    {'1': 'bucket_counts', '3': 6, '4': 3, '5': 4, '10': 'bucketCounts'},
    {'1': 'explicit_bounds', '3': 7, '4': 3, '5': 1, '10': 'explicitBounds'},
    {
      '1': 'attributes',
      '3': 9,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.KeyValue',
      '10': 'attributes'
    },
    {'1': 'min', '3': 11, '4': 1, '5': 1, '10': 'min'},
    {'1': 'max', '3': 12, '4': 1, '5': 1, '10': 'max'},
  ],
};

/// Descriptor for `HistogramDataPoint`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List histogramDataPointDescriptor = $convert.base64Decode(
    'ChJIaXN0b2dyYW1EYXRhUG9pbnQSLwoUc3RhcnRfdGltZV91bml4X25hbm8YAiABKAZSEXN0YX'
    'J0VGltZVVuaXhOYW5vEiQKDnRpbWVfdW5peF9uYW5vGAMgASgGUgx0aW1lVW5peE5hbm8SFAoF'
    'Y291bnQYBCABKARSBWNvdW50EhAKA3N1bRgFIAEoAVIDc3VtEiMKDWJ1Y2tldF9jb3VudHMYBi'
    'ADKARSDGJ1Y2tldENvdW50cxInCg9leHBsaWNpdF9ib3VuZHMYByADKAFSDmV4cGxpY2l0Qm91'
    'bmRzEjMKCmF0dHJpYnV0ZXMYCSADKAsyEy5ob2xvbnMudjEuS2V5VmFsdWVSCmF0dHJpYnV0ZX'
    'MSEAoDbWluGAsgASgBUgNtaW4SEAoDbWF4GAwgASgBUgNtYXg=');
