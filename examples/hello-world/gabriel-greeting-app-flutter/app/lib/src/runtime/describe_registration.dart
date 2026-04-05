import 'dart:io';

import 'package:holons/holons.dart' as holons;
import 'package:holons/gen/holons/v1/describe.pb.dart';
import 'package:path/path.dart' as p;

bool _registered = false;

void ensureAppDescribeRegistered() {
  if (_registered) {
    return;
  }

  final protoDir = _findAppProtoDir();
  if (protoDir == null) {
    throw StateError('failed to locate app proto directory for Describe');
  }

  final response = holons.buildDescribeResponse(protoDir: protoDir);
  if (!response.services.any((service) => service.name == 'holons.v1.CoaxService')) {
    response.services.add(_coaxServiceDoc());
  }
  holons.useStaticResponse(response);
  _registered = true;
}

String? _findAppProtoDir() {
  for (final candidate in _sourceProtoDirCandidates()) {
    if (_containsHolonProto(candidate)) {
      return candidate;
    }
  }
  for (final candidate in _packagedProtoDirCandidates()) {
    if (_containsHolonProto(candidate)) {
      return candidate;
    }
  }
  return null;
}

Iterable<String> _sourceProtoDirCandidates() sync* {
  var current = Directory.current.absolute;
  while (true) {
    yield p.normalize(p.join(current.path, 'api'));

    final parent = current.parent.absolute;
    if (p.equals(parent.path, current.path)) {
      break;
    }
    current = parent;
  }
}

Iterable<String> _packagedProtoDirCandidates() sync* {
  final executable = Platform.resolvedExecutable.trim();
  if (executable.isEmpty) {
    return;
  }

  var current = Directory(p.dirname(executable)).absolute;
  while (true) {
    final currentPath = p.normalize(current.path);
    final currentName = p.basename(currentPath).toLowerCase();

    if (currentName.endsWith('.app')) {
      yield p.join(currentPath, 'Contents', 'Resources', 'AppProto');
    }

    yield p.join(currentPath, 'data', 'AppProto');
    yield p.join(currentPath, 'AppProto');

    final parent = current.parent.absolute;
    if (p.equals(parent.path, current.path)) {
      break;
    }
    current = parent;
  }
}

bool _containsHolonProto(String protoDir) {
  return File(p.join(protoDir, 'v1', 'holon.proto')).existsSync();
}

ServiceDoc _coaxServiceDoc() {
  return ServiceDoc(
    name: 'holons.v1.CoaxService',
    description:
        "COAX interaction surface for the Gabriel Greeting app. It exposes member discovery, connection, and app-level orchestration through the same shared state the UI uses.",
    methods: <MethodDoc>[
      MethodDoc(
        name: 'ListMembers',
        description:
            "List the organism's available member holons, mirroring the holon picker in the UI.",
        inputType: 'holons.v1.ListMembersRequest',
        outputType: 'holons.v1.ListMembersResponse',
        outputFields: <FieldDoc>[
          FieldDoc(
            name: 'members',
            type: 'holons.v1.MemberInfo',
            number: 1,
            description: 'Member holons currently available to the app.',
            label: FieldLabel.FIELD_LABEL_REPEATED,
            nestedFields: _memberInfoFields(),
          ),
        ],
        exampleInput: '{}',
      ),
      MethodDoc(
        name: 'MemberStatus',
        description: 'Query the runtime status of a specific member holon.',
        inputType: 'holons.v1.MemberStatusRequest',
        outputType: 'holons.v1.MemberStatusResponse',
        inputFields: <FieldDoc>[
          FieldDoc(
            name: 'slug',
            type: 'string',
            number: 1,
            description: 'Slug of the member holon to inspect.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            required: true,
            example: '"gabriel-greeting-rust"',
          ),
        ],
        outputFields: <FieldDoc>[
          FieldDoc(
            name: 'member',
            type: 'holons.v1.MemberInfo',
            number: 1,
            description: 'Runtime information for the selected member holon.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            nestedFields: _memberInfoFields(),
          ),
        ],
        exampleInput: '{"slug":"gabriel-greeting-rust"}',
      ),
      MethodDoc(
        name: 'ConnectMember',
        description:
            "Connect a member holon using the app's runtime state, identical to selecting it in the UI.",
        inputType: 'holons.v1.ConnectMemberRequest',
        outputType: 'holons.v1.ConnectMemberResponse',
        inputFields: <FieldDoc>[
          FieldDoc(
            name: 'slug',
            type: 'string',
            number: 1,
            description: 'Slug of the member holon to connect.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            required: true,
            example: '"gabriel-greeting-rust"',
          ),
          FieldDoc(
            name: 'transport',
            type: 'string',
            number: 2,
            description:
                'Optional transport override such as stdio, tcp, or unix.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            example: '"stdio"',
          ),
        ],
        outputFields: <FieldDoc>[
          FieldDoc(
            name: 'member',
            type: 'holons.v1.MemberInfo',
            number: 1,
            description:
                'Runtime information for the member after the connection attempt.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            nestedFields: _memberInfoFields(),
          ),
        ],
        exampleInput:
            '{"slug":"gabriel-greeting-rust","transport":"stdio"}',
      ),
      MethodDoc(
        name: 'DisconnectMember',
        description: 'Disconnect the currently selected member holon.',
        inputType: 'holons.v1.DisconnectMemberRequest',
        outputType: 'holons.v1.DisconnectMemberResponse',
        inputFields: <FieldDoc>[
          FieldDoc(
            name: 'slug',
            type: 'string',
            number: 1,
            description: 'Slug of the member holon to disconnect.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            required: true,
            example: '"gabriel-greeting-rust"',
          ),
        ],
        exampleInput: '{"slug":"gabriel-greeting-rust"}',
      ),
      MethodDoc(
        name: 'Tell',
        description:
            'Forward an RPC command to a member holon through the organism-level COAX surface.',
        inputType: 'holons.v1.TellRequest',
        outputType: 'holons.v1.TellResponse',
        inputFields: <FieldDoc>[
          FieldDoc(
            name: 'member_slug',
            type: 'string',
            number: 1,
            description: 'Member holon slug to address.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            required: true,
            example: '"gabriel-greeting-rust"',
          ),
          FieldDoc(
            name: 'method',
            type: 'string',
            number: 2,
            description: 'Fully qualified RPC method name.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            required: true,
            example: '"greeting.v1.GreetingService/SayHello"',
          ),
          FieldDoc(
            name: 'payload',
            type: 'bytes',
            number: 3,
            description: 'JSON request payload encoded as raw bytes.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
            required: true,
          ),
        ],
        outputFields: <FieldDoc>[
          FieldDoc(
            name: 'payload',
            type: 'bytes',
            number: 1,
            description: 'JSON response payload encoded as raw bytes.',
            label: FieldLabel.FIELD_LABEL_OPTIONAL,
          ),
        ],
      ),
      MethodDoc(
        name: 'TurnOffCoax',
        description: 'Shut down the COAX server gracefully.',
        inputType: 'holons.v1.TurnOffCoaxRequest',
        outputType: 'holons.v1.TurnOffCoaxResponse',
        exampleInput: '{}',
      ),
    ],
  );
}

List<FieldDoc> _memberInfoFields() {
  return <FieldDoc>[
    FieldDoc(
      name: 'slug',
      type: 'string',
      number: 1,
      description: "The member holon's slug.",
      label: FieldLabel.FIELD_LABEL_OPTIONAL,
    ),
    FieldDoc(
      name: 'identity',
      type: 'holons.v1.HolonManifest.Identity',
      number: 2,
      description: 'Identity information exposed by the member holon.',
      label: FieldLabel.FIELD_LABEL_OPTIONAL,
      nestedFields: _identityFields(),
    ),
    FieldDoc(
      name: 'state',
      type: 'holons.v1.MemberState',
      number: 3,
      description: 'Current runtime state for the member holon.',
      label: FieldLabel.FIELD_LABEL_OPTIONAL,
      enumValues: _memberStateValues(),
    ),
    FieldDoc(
      name: 'is_organism',
      type: 'bool',
      number: 4,
      description: 'Whether this member is itself an organism exposing COAX.',
      label: FieldLabel.FIELD_LABEL_OPTIONAL,
    ),
  ];
}

List<FieldDoc> _identityFields() {
  return <FieldDoc>[
    FieldDoc(
      name: 'given_name',
      type: 'string',
      number: 3,
      description: "Given name from the member holon's identity.",
      label: FieldLabel.FIELD_LABEL_OPTIONAL,
      example: '"Gabriel"',
    ),
    FieldDoc(
      name: 'family_name',
      type: 'string',
      number: 4,
      description: "Family name from the member holon's identity.",
      label: FieldLabel.FIELD_LABEL_OPTIONAL,
      example: '"Greeting-Rust"',
    ),
    FieldDoc(
      name: 'motto',
      type: 'string',
      number: 5,
      description: 'Short descriptive motto for the member holon.',
      label: FieldLabel.FIELD_LABEL_OPTIONAL,
    ),
  ];
}

List<EnumValueDoc> _memberStateValues() {
  return <EnumValueDoc>[
    EnumValueDoc(
      name: 'MEMBER_STATE_UNSPECIFIED',
      number: 0,
      description: 'No member state has been established yet.',
    ),
    EnumValueDoc(
      name: 'MEMBER_STATE_AVAILABLE',
      number: 1,
      description: 'Known to the app but not running.',
    ),
    EnumValueDoc(
      name: 'MEMBER_STATE_CONNECTING',
      number: 2,
      description: 'Process starting, not yet ready.',
    ),
    EnumValueDoc(
      name: 'MEMBER_STATE_CONNECTED',
      number: 3,
      description: 'Connected and ready for RPC.',
    ),
    EnumValueDoc(
      name: 'MEMBER_STATE_ERROR',
      number: 4,
      description: 'Connection or process error.',
    ),
  ];
}
