// This is a generated file - do not edit.
//
// Generated from holons/v1/manifest.proto.

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

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest$json = {
  '1': 'HolonManifest',
  '2': [
    {
      '1': 'identity',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Identity',
      '10': 'identity'
    },
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
    {'1': 'lang', '3': 4, '4': 1, '5': 9, '10': 'lang'},
    {
      '1': 'skills',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Skill',
      '10': 'skills'
    },
    {
      '1': 'contract',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Contract',
      '10': 'contract'
    },
    {'1': 'kind', '3': 7, '4': 1, '5': 9, '10': 'kind'},
    {'1': 'platforms', '3': 8, '4': 3, '5': 9, '10': 'platforms'},
    {'1': 'transport', '3': 9, '4': 1, '5': 9, '10': 'transport'},
    {
      '1': 'build',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Build',
      '10': 'build'
    },
    {
      '1': 'requires',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Requires',
      '10': 'requires'
    },
    {
      '1': 'artifacts',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Artifacts',
      '10': 'artifacts'
    },
    {
      '1': 'sequences',
      '3': 14,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Sequence',
      '10': 'sequences'
    },
    {'1': 'guide', '3': 15, '4': 1, '5': 9, '10': 'guide'},
  ],
  '3': [
    HolonManifest_Identity$json,
    HolonManifest_Skill$json,
    HolonManifest_Sequence$json,
    HolonManifest_Contract$json,
    HolonManifest_Build$json,
    HolonManifest_Step$json,
    HolonManifest_Requires$json,
    HolonManifest_Artifacts$json
  ],
  '9': [
    {'1': 2, '2': 3},
    {'1': 12, '2': 13},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Identity$json = {
  '1': 'Identity',
  '2': [
    {'1': 'schema', '3': 1, '4': 1, '5': 9, '10': 'schema'},
    {'1': 'uuid', '3': 2, '4': 1, '5': 9, '10': 'uuid'},
    {'1': 'given_name', '3': 3, '4': 1, '5': 9, '10': 'givenName'},
    {'1': 'family_name', '3': 4, '4': 1, '5': 9, '10': 'familyName'},
    {'1': 'motto', '3': 5, '4': 1, '5': 9, '10': 'motto'},
    {'1': 'composer', '3': 6, '4': 1, '5': 9, '10': 'composer'},
    {'1': 'status', '3': 8, '4': 1, '5': 9, '10': 'status'},
    {'1': 'born', '3': 9, '4': 1, '5': 9, '10': 'born'},
    {'1': 'version', '3': 10, '4': 1, '5': 9, '10': 'version'},
    {'1': 'aliases', '3': 11, '4': 3, '5': 9, '10': 'aliases'},
  ],
  '9': [
    {'1': 7, '2': 8},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Skill$json = {
  '1': 'Skill',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {'1': 'when', '3': 3, '4': 1, '5': 9, '10': 'when'},
    {'1': 'steps', '3': 4, '4': 3, '5': 9, '10': 'steps'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Sequence$json = {
  '1': 'Sequence',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'params',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Sequence.Param',
      '10': 'params'
    },
    {'1': 'steps', '3': 4, '4': 3, '5': 9, '10': 'steps'},
  ],
  '3': [HolonManifest_Sequence_Param$json],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Sequence_Param$json = {
  '1': 'Param',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {'1': 'required', '3': 3, '4': 1, '5': 8, '10': 'required'},
    {'1': 'default', '3': 4, '4': 1, '5': 9, '10': 'default'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Contract$json = {
  '1': 'Contract',
  '2': [
    {'1': 'proto', '3': 1, '4': 1, '5': 9, '10': 'proto'},
    {'1': 'service', '3': 2, '4': 1, '5': 9, '10': 'service'},
    {'1': 'rpcs', '3': 3, '4': 3, '5': 9, '10': 'rpcs'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Build$json = {
  '1': 'Build',
  '2': [
    {'1': 'runner', '3': 1, '4': 1, '5': 9, '10': 'runner'},
    {'1': 'main', '3': 2, '4': 1, '5': 9, '10': 'main'},
    {
      '1': 'defaults',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Build.Defaults',
      '10': 'defaults'
    },
    {
      '1': 'members',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Build.Member',
      '10': 'members'
    },
    {
      '1': 'targets',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Build.TargetsEntry',
      '10': 'targets'
    },
    {'1': 'templates', '3': 6, '4': 3, '5': 9, '10': 'templates'},
  ],
  '3': [
    HolonManifest_Build_TargetsEntry$json,
    HolonManifest_Build_Defaults$json,
    HolonManifest_Build_Member$json,
    HolonManifest_Build_Target$json
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Build_TargetsEntry$json = {
  '1': 'TargetsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Build.Target',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Build_Defaults$json = {
  '1': 'Defaults',
  '2': [
    {'1': 'target', '3': 1, '4': 1, '5': 9, '10': 'target'},
    {'1': 'mode', '3': 2, '4': 1, '5': 9, '10': 'mode'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Build_Member$json = {
  '1': 'Member',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'path', '3': 2, '4': 1, '5': 9, '10': 'path'},
    {'1': 'type', '3': 3, '4': 1, '5': 9, '10': 'type'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Build_Target$json = {
  '1': 'Target',
  '2': [
    {
      '1': 'steps',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Step',
      '10': 'steps'
    },
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Step$json = {
  '1': 'Step',
  '2': [
    {
      '1': 'exec',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Step.Exec',
      '9': 0,
      '10': 'exec'
    },
    {
      '1': 'copy',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Step.Copy',
      '9': 0,
      '10': 'copy'
    },
    {'1': 'build_member', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'buildMember'},
    {
      '1': 'assert_file',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Step.AssertFile',
      '9': 0,
      '10': 'assertFile'
    },
    {
      '1': 'copy_artifact',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Step.CopyArtifact',
      '9': 0,
      '10': 'copyArtifact'
    },
  ],
  '3': [
    HolonManifest_Step_Exec$json,
    HolonManifest_Step_Copy$json,
    HolonManifest_Step_AssertFile$json,
    HolonManifest_Step_CopyArtifact$json
  ],
  '8': [
    {'1': 'action'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Step_Exec$json = {
  '1': 'Exec',
  '2': [
    {'1': 'cwd', '3': 1, '4': 1, '5': 9, '10': 'cwd'},
    {'1': 'argv', '3': 2, '4': 3, '5': 9, '10': 'argv'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Step_Copy$json = {
  '1': 'Copy',
  '2': [
    {'1': 'from', '3': 1, '4': 1, '5': 9, '10': 'from'},
    {'1': 'to', '3': 2, '4': 1, '5': 9, '10': 'to'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Step_AssertFile$json = {
  '1': 'AssertFile',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Step_CopyArtifact$json = {
  '1': 'CopyArtifact',
  '2': [
    {'1': 'from', '3': 1, '4': 1, '5': 9, '10': 'from'},
    {'1': 'to', '3': 2, '4': 1, '5': 9, '10': 'to'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Requires$json = {
  '1': 'Requires',
  '2': [
    {'1': 'commands', '3': 1, '4': 3, '5': 9, '10': 'commands'},
    {'1': 'files', '3': 2, '4': 3, '5': 9, '10': 'files'},
    {'1': 'platforms', '3': 3, '4': 3, '5': 9, '10': 'platforms'},
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Artifacts$json = {
  '1': 'Artifacts',
  '2': [
    {'1': 'binary', '3': 1, '4': 1, '5': 9, '10': 'binary'},
    {'1': 'primary', '3': 2, '4': 1, '5': 9, '10': 'primary'},
    {
      '1': 'by_target',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Artifacts.ByTargetEntry',
      '10': 'byTarget'
    },
  ],
  '3': [
    HolonManifest_Artifacts_ByTargetEntry$json,
    HolonManifest_Artifacts_TargetArtifacts$json
  ],
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Artifacts_ByTargetEntry$json = {
  '1': 'ByTargetEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.holons.v1.HolonManifest.Artifacts.TargetArtifacts',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

@$core.Deprecated('Use holonManifestDescriptor instead')
const HolonManifest_Artifacts_TargetArtifacts$json = {
  '1': 'TargetArtifacts',
  '2': [
    {'1': 'debug', '3': 1, '4': 1, '5': 9, '10': 'debug'},
    {'1': 'release', '3': 2, '4': 1, '5': 9, '10': 'release'},
    {'1': 'profile', '3': 3, '4': 1, '5': 9, '10': 'profile'},
  ],
};

/// Descriptor for `HolonManifest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List holonManifestDescriptor = $convert.base64Decode(
    'Cg1Ib2xvbk1hbmlmZXN0Ej0KCGlkZW50aXR5GAEgASgLMiEuaG9sb25zLnYxLkhvbG9uTWFuaW'
    'Zlc3QuSWRlbnRpdHlSCGlkZW50aXR5EiAKC2Rlc2NyaXB0aW9uGAMgASgJUgtkZXNjcmlwdGlv'
    'bhISCgRsYW5nGAQgASgJUgRsYW5nEjYKBnNraWxscxgFIAMoCzIeLmhvbG9ucy52MS5Ib2xvbk'
    '1hbmlmZXN0LlNraWxsUgZza2lsbHMSPQoIY29udHJhY3QYBiABKAsyIS5ob2xvbnMudjEuSG9s'
    'b25NYW5pZmVzdC5Db250cmFjdFIIY29udHJhY3QSEgoEa2luZBgHIAEoCVIEa2luZBIcCglwbG'
    'F0Zm9ybXMYCCADKAlSCXBsYXRmb3JtcxIcCgl0cmFuc3BvcnQYCSABKAlSCXRyYW5zcG9ydBI0'
    'CgVidWlsZBgKIAEoCzIeLmhvbG9ucy52MS5Ib2xvbk1hbmlmZXN0LkJ1aWxkUgVidWlsZBI9Cg'
    'hyZXF1aXJlcxgLIAEoCzIhLmhvbG9ucy52MS5Ib2xvbk1hbmlmZXN0LlJlcXVpcmVzUghyZXF1'
    'aXJlcxJACglhcnRpZmFjdHMYDSABKAsyIi5ob2xvbnMudjEuSG9sb25NYW5pZmVzdC5BcnRpZm'
    'FjdHNSCWFydGlmYWN0cxI/CglzZXF1ZW5jZXMYDiADKAsyIS5ob2xvbnMudjEuSG9sb25NYW5p'
    'ZmVzdC5TZXF1ZW5jZVIJc2VxdWVuY2VzEhQKBWd1aWRlGA8gASgJUgVndWlkZRqOAgoISWRlbn'
    'RpdHkSFgoGc2NoZW1hGAEgASgJUgZzY2hlbWESEgoEdXVpZBgCIAEoCVIEdXVpZBIdCgpnaXZl'
    'bl9uYW1lGAMgASgJUglnaXZlbk5hbWUSHwoLZmFtaWx5X25hbWUYBCABKAlSCmZhbWlseU5hbW'
    'USFAoFbW90dG8YBSABKAlSBW1vdHRvEhoKCGNvbXBvc2VyGAYgASgJUghjb21wb3NlchIWCgZz'
    'dGF0dXMYCCABKAlSBnN0YXR1cxISCgRib3JuGAkgASgJUgRib3JuEhgKB3ZlcnNpb24YCiABKA'
    'lSB3ZlcnNpb24SGAoHYWxpYXNlcxgLIAMoCVIHYWxpYXNlc0oECAcQCBpnCgVTa2lsbBISCgRu'
    'YW1lGAEgASgJUgRuYW1lEiAKC2Rlc2NyaXB0aW9uGAIgASgJUgtkZXNjcmlwdGlvbhISCgR3aG'
    'VuGAMgASgJUgR3aGVuEhQKBXN0ZXBzGAQgAygJUgVzdGVwcxqMAgoIU2VxdWVuY2USEgoEbmFt'
    'ZRgBIAEoCVIEbmFtZRIgCgtkZXNjcmlwdGlvbhgCIAEoCVILZGVzY3JpcHRpb24SPwoGcGFyYW'
    '1zGAMgAygLMicuaG9sb25zLnYxLkhvbG9uTWFuaWZlc3QuU2VxdWVuY2UuUGFyYW1SBnBhcmFt'
    'cxIUCgVzdGVwcxgEIAMoCVIFc3RlcHMacwoFUGFyYW0SEgoEbmFtZRgBIAEoCVIEbmFtZRIgCg'
    'tkZXNjcmlwdGlvbhgCIAEoCVILZGVzY3JpcHRpb24SGgoIcmVxdWlyZWQYAyABKAhSCHJlcXVp'
    'cmVkEhgKB2RlZmF1bHQYBCABKAlSB2RlZmF1bHQaTgoIQ29udHJhY3QSFAoFcHJvdG8YASABKA'
    'lSBXByb3RvEhgKB3NlcnZpY2UYAiABKAlSB3NlcnZpY2USEgoEcnBjcxgDIAMoCVIEcnBjcxq6'
    'BAoFQnVpbGQSFgoGcnVubmVyGAEgASgJUgZydW5uZXISEgoEbWFpbhgCIAEoCVIEbWFpbhJDCg'
    'hkZWZhdWx0cxgDIAEoCzInLmhvbG9ucy52MS5Ib2xvbk1hbmlmZXN0LkJ1aWxkLkRlZmF1bHRz'
    'UghkZWZhdWx0cxI/CgdtZW1iZXJzGAQgAygLMiUuaG9sb25zLnYxLkhvbG9uTWFuaWZlc3QuQn'
    'VpbGQuTWVtYmVyUgdtZW1iZXJzEkUKB3RhcmdldHMYBSADKAsyKy5ob2xvbnMudjEuSG9sb25N'
    'YW5pZmVzdC5CdWlsZC5UYXJnZXRzRW50cnlSB3RhcmdldHMSHAoJdGVtcGxhdGVzGAYgAygJUg'
    'l0ZW1wbGF0ZXMaYQoMVGFyZ2V0c0VudHJ5EhAKA2tleRgBIAEoCVIDa2V5EjsKBXZhbHVlGAIg'
    'ASgLMiUuaG9sb25zLnYxLkhvbG9uTWFuaWZlc3QuQnVpbGQuVGFyZ2V0UgV2YWx1ZToCOAEaNg'
    'oIRGVmYXVsdHMSFgoGdGFyZ2V0GAEgASgJUgZ0YXJnZXQSEgoEbW9kZRgCIAEoCVIEbW9kZRpA'
    'CgZNZW1iZXISDgoCaWQYASABKAlSAmlkEhIKBHBhdGgYAiABKAlSBHBhdGgSEgoEdHlwZRgDIA'
    'EoCVIEdHlwZRo9CgZUYXJnZXQSMwoFc3RlcHMYASADKAsyHS5ob2xvbnMudjEuSG9sb25NYW5p'
    'ZmVzdC5TdGVwUgVzdGVwcxr5AwoEU3RlcBI4CgRleGVjGAEgASgLMiIuaG9sb25zLnYxLkhvbG'
    '9uTWFuaWZlc3QuU3RlcC5FeGVjSABSBGV4ZWMSOAoEY29weRgCIAEoCzIiLmhvbG9ucy52MS5I'
    'b2xvbk1hbmlmZXN0LlN0ZXAuQ29weUgAUgRjb3B5EiMKDGJ1aWxkX21lbWJlchgDIAEoCUgAUg'
    'tidWlsZE1lbWJlchJLCgthc3NlcnRfZmlsZRgEIAEoCzIoLmhvbG9ucy52MS5Ib2xvbk1hbmlm'
    'ZXN0LlN0ZXAuQXNzZXJ0RmlsZUgAUgphc3NlcnRGaWxlElEKDWNvcHlfYXJ0aWZhY3QYBSABKA'
    'syKi5ob2xvbnMudjEuSG9sb25NYW5pZmVzdC5TdGVwLkNvcHlBcnRpZmFjdEgAUgxjb3B5QXJ0'
    'aWZhY3QaLAoERXhlYxIQCgNjd2QYASABKAlSA2N3ZBISCgRhcmd2GAIgAygJUgRhcmd2GioKBE'
    'NvcHkSEgoEZnJvbRgBIAEoCVIEZnJvbRIOCgJ0bxgCIAEoCVICdG8aIAoKQXNzZXJ0RmlsZRIS'
    'CgRwYXRoGAEgASgJUgRwYXRoGjIKDENvcHlBcnRpZmFjdBISCgRmcm9tGAEgASgJUgRmcm9tEg'
    '4KAnRvGAIgASgJUgJ0b0IICgZhY3Rpb24aWgoIUmVxdWlyZXMSGgoIY29tbWFuZHMYASADKAlS'
    'CGNvbW1hbmRzEhQKBWZpbGVzGAIgAygJUgVmaWxlcxIcCglwbGF0Zm9ybXMYAyADKAlSCXBsYX'
    'Rmb3JtcxraAgoJQXJ0aWZhY3RzEhYKBmJpbmFyeRgBIAEoCVIGYmluYXJ5EhgKB3ByaW1hcnkY'
    'AiABKAlSB3ByaW1hcnkSTQoJYnlfdGFyZ2V0GAMgAygLMjAuaG9sb25zLnYxLkhvbG9uTWFuaW'
    'Zlc3QuQXJ0aWZhY3RzLkJ5VGFyZ2V0RW50cnlSCGJ5VGFyZ2V0Gm8KDUJ5VGFyZ2V0RW50cnkS'
    'EAoDa2V5GAEgASgJUgNrZXkSSAoFdmFsdWUYAiABKAsyMi5ob2xvbnMudjEuSG9sb25NYW5pZm'
    'VzdC5BcnRpZmFjdHMuVGFyZ2V0QXJ0aWZhY3RzUgV2YWx1ZToCOAEaWwoPVGFyZ2V0QXJ0aWZh'
    'Y3RzEhQKBWRlYnVnGAEgASgJUgVkZWJ1ZxIYCgdyZWxlYXNlGAIgASgJUgdyZWxlYXNlEhgKB3'
    'Byb2ZpbGUYAyABKAlSB3Byb2ZpbGVKBAgCEANKBAgMEA0=');
