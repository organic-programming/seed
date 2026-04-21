import 'dart:io';

import 'package:grpc/grpc.dart';

import '../gen/holons/v1/describe.pbgrpc.dart';
import '../gen/holons/v1/manifest.pb.dart';
import 'identity.dart';

const _holonMetaService = 'holons.v1.HolonMeta';
const String errNoIncodeDescription =
    'no Incode Description registered — run op build';

DescribeResponse? _staticDescribeResponse;

final RegExp _packagePattern = RegExp(r'^package\s+([A-Za-z0-9_.]+)\s*;');
final RegExp _servicePattern =
    RegExp(r'^service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?');
final RegExp _messagePattern =
    RegExp(r'^message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?');
final RegExp _enumPattern = RegExp(r'^enum\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?');
final RegExp _rpcPattern = RegExp(
  r'^rpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*returns\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*;?',
);
final RegExp _mapFieldPattern = RegExp(
  r'^(repeated\s+)?map\s*<\s*([.A-Za-z0-9_]+)\s*,\s*([.A-Za-z0-9_]+)\s*>\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;',
);
final RegExp _fieldPattern = RegExp(
  r'^(optional\s+|repeated\s+)?([.A-Za-z0-9_]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;',
);
final RegExp _enumValuePattern =
    RegExp(r'^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(-?\d+)\s*;');
final Set<String> _scalarTypes = <String>{
  'double',
  'float',
  'int64',
  'uint64',
  'int32',
  'fixed64',
  'fixed32',
  'bool',
  'string',
  'bytes',
  'uint32',
  'sfixed32',
  'sfixed64',
  'sint32',
  'sint64',
};

void useStaticResponse(DescribeResponse? response) {
  _staticDescribeResponse = _cloneDescribeResponse(response);
}

Service register() {
  final response = _registeredStaticResponse();
  if (response == null) {
    throw const DescribeRegistrationException(errNoIncodeDescription);
  }
  return HolonMetaDescribeService._(response);
}

Service registerService() => register();

class DescribeRegistrationException implements Exception {
  const DescribeRegistrationException(this.message);

  final String message;

  @override
  String toString() => message;
}

DescribeResponse buildDescribeResponse({
  required String protoDir,
  String? manifestPath,
}) {
  final resolved =
      resolveProtoFile(manifestPath ?? resolveManifestPath(protoDir));
  final index = _parseProtoDirectory(protoDir);

  return DescribeResponse()
    ..manifest = _protoManifest(resolved)
    ..services.addAll(
      index.services
          .where((service) => service.fullName != _holonMetaService)
          .map((service) => _serviceDoc(service, index)),
    );
}

HolonMetaDescribeService describeService() =>
    register() as HolonMetaDescribeService;

class HolonMetaDescribeService extends HolonMetaServiceBase {
  final DescribeResponse _response;

  HolonMetaDescribeService._(this._response);

  @override
  Future<DescribeResponse> describe(
    ServiceCall call,
    DescribeRequest request,
  ) async {
    return DescribeResponse.fromBuffer(_response.writeToBuffer());
  }
}

DescribeResponse? _registeredStaticResponse() {
  return _cloneDescribeResponse(_staticDescribeResponse);
}

DescribeResponse? _cloneDescribeResponse(DescribeResponse? response) {
  if (response == null) {
    return null;
  }
  return DescribeResponse.fromBuffer(response.writeToBuffer());
}

HolonManifest _protoManifest(ResolvedManifest resolved) {
  return HolonManifest()
    ..identity = (HolonManifest_Identity()
      ..schema = 'holon/v1'
      ..uuid = resolved.identity.uuid
      ..givenName = resolved.identity.givenName
      ..familyName = resolved.identity.familyName
      ..motto = resolved.identity.motto
      ..composer = resolved.identity.composer
      ..status = resolved.identity.status
      ..born = resolved.identity.born
      ..aliases.addAll(resolved.identity.aliases))
    ..lang = resolved.identity.lang
    ..kind = resolved.kind
    ..build = (HolonManifest_Build()
      ..runner = resolved.buildRunner
      ..main = resolved.buildMain)
    ..artifacts = (HolonManifest_Artifacts()
      ..binary = resolved.artifactBinary
      ..primary = resolved.artifactPrimary);
}

ServiceDoc _serviceDoc(_ServiceDef service, _ProtoIndex index) {
  return ServiceDoc()
    ..name = service.fullName
    ..description = service.comment.description
    ..methods
        .addAll(service.methods.map((method) => _methodDoc(method, index)));
}

MethodDoc _methodDoc(_MethodDef method, _ProtoIndex index) {
  final doc = MethodDoc()
    ..name = method.name
    ..description = method.comment.description
    ..inputType = method.inputType
    ..outputType = method.outputType
    ..clientStreaming = method.clientStreaming
    ..serverStreaming = method.serverStreaming
    ..exampleInput = method.comment.example;

  final input = index.messages[method.inputType];
  if (input != null) {
    doc.inputFields.addAll(
      input.fields.map((field) => _fieldDoc(field, index, <String>{})),
    );
  }

  final output = index.messages[method.outputType];
  if (output != null) {
    doc.outputFields.addAll(
      output.fields.map((field) => _fieldDoc(field, index, <String>{})),
    );
  }

  return doc;
}

FieldDoc _fieldDoc(_FieldDef field, _ProtoIndex index, Set<String> seen) {
  final doc = FieldDoc()
    ..name = field.name
    ..type = field.typeName
    ..number = field.number
    ..description = field.comment.description
    ..label = field.label
    ..required = field.comment.required
    ..example = field.comment.example;

  if (field.mapKeyType != null) {
    doc.mapKeyType = field.mapKeyType!;
  }
  if (field.mapValueType != null) {
    doc.mapValueType = field.mapValueType!;
  }

  if (field.cardinality == _FieldCardinality.map) {
    final mapValueType = field.resolvedMapValueType(index);
    final nested = index.messages[mapValueType];
    if (nested != null && seen.add(nested.fullName)) {
      doc.nestedFields.addAll(
        nested.fields.map(
          (nestedField) =>
              _fieldDoc(nestedField, index, Set<String>.from(seen)),
        ),
      );
    }
    final enumDef = index.enums[mapValueType];
    if (enumDef != null) {
      doc.enumValues.addAll(enumDef.values.map(_enumValueDoc));
    }
    return doc;
  }

  final resolvedType = field.resolvedType(index);
  final nested = index.messages[resolvedType];
  if (nested != null && seen.add(nested.fullName)) {
    doc.nestedFields.addAll(
      nested.fields.map(
        (nestedField) => _fieldDoc(nestedField, index, Set<String>.from(seen)),
      ),
    );
  }
  final enumDef = index.enums[resolvedType];
  if (enumDef != null) {
    doc.enumValues.addAll(enumDef.values.map(_enumValueDoc));
  }

  return doc;
}

EnumValueDoc _enumValueDoc(_EnumValueDef value) {
  return EnumValueDoc()
    ..name = value.name
    ..number = value.number
    ..description = value.comment.description;
}

_ProtoIndex _parseProtoDirectory(String protoDir) {
  final index = _ProtoIndex();
  final directory = Directory(protoDir);
  if (!directory.existsSync()) {
    return index;
  }

  final files = directory
      .listSync(recursive: true)
      .whereType<File>()
      .where((file) => file.path.endsWith('.proto'))
      .map((file) => file.path)
      .toList()
    ..sort();

  for (final file in files) {
    _parseProtoFile(file, index);
  }

  return index;
}

void _parseProtoFile(String path, _ProtoIndex index) {
  final lines = File(path).readAsLinesSync();
  var packageName = '';
  final stack = <_Block>[];
  final pendingComments = <String>[];

  for (final raw in lines) {
    final line = raw.trim();
    if (line.startsWith('//')) {
      pendingComments.add(line.substring(2).trim());
      continue;
    }
    if (line.isEmpty) {
      continue;
    }

    final packageMatch = _packagePattern.firstMatch(line);
    if (packageMatch != null) {
      packageName = packageMatch.group(1)!;
      pendingComments.clear();
      continue;
    }

    final serviceMatch = _servicePattern.firstMatch(line);
    if (serviceMatch != null) {
      final service = _ServiceDef(
        fullName: _qualify(packageName, serviceMatch.group(1)!),
        comment: _CommentMeta.parse(pendingComments),
      );
      index.services.add(service);
      pendingComments.clear();
      stack.add(_Block.service(serviceMatch.group(1)!));
      _trimClosedBlocks(line, stack);
      continue;
    }

    final messageMatch = _messagePattern.firstMatch(line);
    if (messageMatch != null) {
      final scope = _messageScope(stack);
      final name = messageMatch.group(1)!;
      final message = _MessageDef(
        fullName: _qualify(packageName, _qualifyScope(scope, name)),
        scope: scope,
      );
      index.messages[message.fullName] = message;
      index.simpleTypes.putIfAbsent(message.simpleKey, () => message.fullName);
      pendingComments.clear();
      stack.add(_Block.message(name));
      _trimClosedBlocks(line, stack);
      continue;
    }

    final enumMatch = _enumPattern.firstMatch(line);
    if (enumMatch != null) {
      final scope = _messageScope(stack);
      final name = enumMatch.group(1)!;
      final enumDef = _EnumDef(
        fullName: _qualify(packageName, _qualifyScope(scope, name)),
        scope: scope,
      );
      index.enums[enumDef.fullName] = enumDef;
      index.simpleTypes.putIfAbsent(enumDef.simpleKey, () => enumDef.fullName);
      pendingComments.clear();
      stack.add(_Block.enumValue(name));
      _trimClosedBlocks(line, stack);
      continue;
    }

    final current = stack.isEmpty ? null : stack.last;
    switch (current?.kind) {
      case _BlockKind.service:
        final rpcMatch = _rpcPattern.firstMatch(line);
        if (rpcMatch != null && index.services.isNotEmpty) {
          index.services.last.methods.add(
            _MethodDef(
              name: rpcMatch.group(1)!,
              inputType: _resolveTypeName(
                  rpcMatch.group(3)!, packageName, const <String>[], index),
              outputType: _resolveTypeName(
                  rpcMatch.group(5)!, packageName, const <String>[], index),
              clientStreaming: rpcMatch.group(2) != null,
              serverStreaming: rpcMatch.group(4) != null,
              comment: _CommentMeta.parse(pendingComments),
            ),
          );
          pendingComments.clear();
          _trimClosedBlocks(line, stack);
          continue;
        }
        break;
      case _BlockKind.message:
        final scope = _messageScope(stack);
        final key = _qualify(packageName, scope.join('.'));
        final mapFieldMatch = _mapFieldPattern.firstMatch(line);
        if (mapFieldMatch != null) {
          index.messages[key]?.fields.add(
            _FieldDef(
              name: mapFieldMatch.group(4)!,
              number: int.parse(mapFieldMatch.group(5)!),
              comment: _CommentMeta.parse(pendingComments),
              cardinality: _FieldCardinality.map,
              type: null,
              mapKeyType: _resolveTypeName(
                  mapFieldMatch.group(2)!, packageName, scope, index),
              mapValueType: _resolveTypeName(
                  mapFieldMatch.group(3)!, packageName, scope, index),
              packageName: packageName,
              scope: scope,
            ),
          );
          pendingComments.clear();
          _trimClosedBlocks(line, stack);
          continue;
        }

        final fieldMatch = _fieldPattern.firstMatch(line);
        if (fieldMatch != null) {
          final qualifier = (fieldMatch.group(1) ?? '').trim();
          index.messages[key]?.fields.add(
            _FieldDef(
              name: fieldMatch.group(3)!,
              number: int.parse(fieldMatch.group(4)!),
              comment: _CommentMeta.parse(pendingComments),
              cardinality: qualifier == 'repeated'
                  ? _FieldCardinality.repeated
                  : _FieldCardinality.optional,
              type: _resolveTypeName(
                  fieldMatch.group(2)!, packageName, scope, index),
              mapKeyType: null,
              mapValueType: null,
              packageName: packageName,
              scope: scope,
            ),
          );
          pendingComments.clear();
          _trimClosedBlocks(line, stack);
          continue;
        }
        break;
      case _BlockKind.enumType:
        final enumKey = _qualify(
          packageName,
          _qualifyScope(_messageScope(stack), current!.name),
        );
        final enumValueMatch = _enumValuePattern.firstMatch(line);
        if (enumValueMatch != null) {
          index.enums[enumKey]?.values.add(
            _EnumValueDef(
              name: enumValueMatch.group(1)!,
              number: int.parse(enumValueMatch.group(2)!),
              comment: _CommentMeta.parse(pendingComments),
            ),
          );
          pendingComments.clear();
          _trimClosedBlocks(line, stack);
          continue;
        }
        break;
      case null:
        break;
    }

    pendingComments.clear();
    _trimClosedBlocks(line, stack);
  }
}

void _trimClosedBlocks(String line, List<_Block> stack) {
  final closers = '}'.allMatches(line).length;
  for (var i = 0; i < closers && stack.isNotEmpty; i++) {
    stack.removeLast();
  }
}

List<String> _messageScope(List<_Block> stack) => stack
    .where((block) => block.kind == _BlockKind.message)
    .map((block) => block.name)
    .toList(growable: false);

String _qualify(String packageName, String name) {
  if (name.isEmpty) {
    return '';
  }
  final cleaned = name.startsWith('.') ? name.substring(1) : name;
  if (cleaned.contains('.') || packageName.isEmpty) {
    return cleaned;
  }
  return '$packageName.$cleaned';
}

String _qualifyScope(List<String> scope, String name) =>
    scope.isEmpty ? name : '${scope.join('.')}.$name';

String _resolveTypeName(
  String typeName,
  String packageName,
  List<String> scope,
  _ProtoIndex index,
) {
  final cleaned = typeName.trim();
  if (cleaned.isEmpty) {
    return '';
  }
  if (cleaned.startsWith('.')) {
    return cleaned.substring(1);
  }
  if (_scalarTypes.contains(cleaned)) {
    return cleaned;
  }
  if (cleaned.contains('.')) {
    final qualified = _qualify(packageName, cleaned);
    if (index.messages.containsKey(qualified) ||
        index.enums.containsKey(qualified)) {
      return qualified;
    }
    return cleaned;
  }

  for (var i = scope.length; i >= 0; i--) {
    final candidate =
        _qualify(packageName, _qualifyScope(scope.take(i).toList(), cleaned));
    if (index.messages.containsKey(candidate) ||
        index.enums.containsKey(candidate)) {
      return candidate;
    }
  }
  final nested = index.simpleTypes[_qualifyScope(scope, cleaned)];
  if (nested != null) {
    return nested;
  }
  final direct = index.simpleTypes[cleaned];
  if (direct != null) {
    return direct;
  }
  return _qualify(packageName, cleaned);
}

class _ProtoIndex {
  final List<_ServiceDef> services = <_ServiceDef>[];
  final Map<String, _MessageDef> messages = <String, _MessageDef>{};
  final Map<String, _EnumDef> enums = <String, _EnumDef>{};
  final Map<String, String> simpleTypes = <String, String>{};
}

class _ServiceDef {
  _ServiceDef({
    required this.fullName,
    required this.comment,
  });

  final String fullName;
  final _CommentMeta comment;
  final List<_MethodDef> methods = <_MethodDef>[];
}

class _MethodDef {
  _MethodDef({
    required this.name,
    required this.inputType,
    required this.outputType,
    required this.clientStreaming,
    required this.serverStreaming,
    required this.comment,
  });

  final String name;
  final String inputType;
  final String outputType;
  final bool clientStreaming;
  final bool serverStreaming;
  final _CommentMeta comment;
}

class _MessageDef {
  _MessageDef({
    required this.fullName,
    required this.scope,
  });

  final String fullName;
  final List<String> scope;
  final List<_FieldDef> fields = <_FieldDef>[];

  String get simpleKey => _qualifyScope(scope, fullName.split('.').last);
}

class _EnumDef {
  _EnumDef({
    required this.fullName,
    required this.scope,
  });

  final String fullName;
  final List<String> scope;
  final List<_EnumValueDef> values = <_EnumValueDef>[];

  String get simpleKey => _qualifyScope(scope, fullName.split('.').last);
}

class _EnumValueDef {
  _EnumValueDef({
    required this.name,
    required this.number,
    required this.comment,
  });

  final String name;
  final int number;
  final _CommentMeta comment;
}

enum _FieldCardinality { optional, repeated, map }

class _FieldDef {
  _FieldDef({
    required this.name,
    required this.number,
    required this.comment,
    required this.cardinality,
    required this.type,
    required this.mapKeyType,
    required this.mapValueType,
    required this.packageName,
    required this.scope,
  });

  final String name;
  final int number;
  final _CommentMeta comment;
  final _FieldCardinality cardinality;
  final String? type;
  final String? mapKeyType;
  final String? mapValueType;
  final String packageName;
  final List<String> scope;

  String get typeName => cardinality == _FieldCardinality.map
      ? 'map<$mapKeyType, $mapValueType>'
      : (type ?? '');

  FieldLabel get label {
    switch (cardinality) {
      case _FieldCardinality.repeated:
        return FieldLabel.FIELD_LABEL_REPEATED;
      case _FieldCardinality.map:
        return FieldLabel.FIELD_LABEL_MAP;
      case _FieldCardinality.optional:
        return FieldLabel.FIELD_LABEL_OPTIONAL;
    }
  }

  String resolvedType(_ProtoIndex index) =>
      _resolveTypeName(type ?? '', packageName, scope, index);

  String resolvedMapValueType(_ProtoIndex index) =>
      _resolveTypeName(mapValueType ?? '', packageName, scope, index);
}

class _CommentMeta {
  _CommentMeta({
    required this.description,
    required this.required,
    required this.example,
  });

  final String description;
  final bool required;
  final String example;

  factory _CommentMeta.parse(List<String> lines) {
    final description = <String>[];
    final examples = <String>[];
    var required = false;

    for (final raw in lines) {
      final line = raw.trim();
      if (line.isEmpty) {
        continue;
      }
      if (line == '@required') {
        required = true;
        continue;
      }
      if (line.startsWith('@example')) {
        final example = line.substring('@example'.length).trim();
        if (example.isNotEmpty) {
          examples.add(example);
        }
        continue;
      }
      description.add(line);
    }

    return _CommentMeta(
      description: description.join(' '),
      required: required,
      example: examples.join('\n'),
    );
  }
}

enum _BlockKind { service, message, enumType }

class _Block {
  _Block._(this.kind, this.name);

  final _BlockKind kind;
  final String name;

  factory _Block.service(String name) => _Block._(_BlockKind.service, name);
  factory _Block.message(String name) => _Block._(_BlockKind.message, name);
  factory _Block.enumValue(String name) => _Block._(_BlockKind.enumType, name);
}
