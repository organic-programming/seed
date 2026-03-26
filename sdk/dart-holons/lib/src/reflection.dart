import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:protoc_plugin/src/gen/google/protobuf/descriptor.pb.dart'
    as descriptor;

import 'gen/grpc/reflection/v1alpha/reflection.pbgrpc.dart';

ServerReflectionService reflectionService({
  required String protoDir,
}) {
  return ServerReflectionService._(_ReflectionCatalog.build(protoDir));
}

class ServerReflectionService extends ServerReflectionServiceBase {
  ServerReflectionService._(this._catalog);

  final _ReflectionCatalog _catalog;

  @override
  Stream<ServerReflectionResponse> serverReflectionInfo(
    ServiceCall call,
    Stream<ServerReflectionRequest> request,
  ) async* {
    call;
    await for (final message in request) {
      yield _catalog.response(message);
    }
  }
}

class _ReflectionCatalog {
  _ReflectionCatalog({
    required this.services,
    required this.filesByName,
    required this.fileBytesByName,
    required this.symbolsToFileNames,
  });

  final List<String> services;
  final Map<String, descriptor.FileDescriptorProto> filesByName;
  final Map<String, List<int>> fileBytesByName;
  final Map<String, String> symbolsToFileNames;

  factory _ReflectionCatalog.build(String protoDir) {
    final descriptorSet = _loadDescriptorSet(protoDir);
    final filesByName = <String, descriptor.FileDescriptorProto>{};
    final fileBytesByName = <String, List<int>>{};
    final symbolsToFileNames = <String, String>{};
    final services = <String>{};

    for (final file in descriptorSet.file) {
      if (file.name.isEmpty) {
        continue;
      }
      filesByName[file.name] = file;
      fileBytesByName[file.name] = file.writeToBuffer();

      for (final service in file.service) {
        final serviceName = _qualify(file.package, service.name);
        if (serviceName.isEmpty) {
          continue;
        }
        services.add(serviceName);
        symbolsToFileNames[serviceName] = file.name;
        for (final method in service.method) {
          if (method.name.isNotEmpty) {
            symbolsToFileNames['$serviceName.${method.name}'] = file.name;
          }
        }
      }

      for (final message in file.messageType) {
        _indexMessage(
          message,
          prefix: file.package,
          fileName: file.name,
          symbolsToFileNames: symbolsToFileNames,
        );
      }
      for (final enumType in file.enumType) {
        final enumName = _qualify(file.package, enumType.name);
        if (enumName.isNotEmpty) {
          symbolsToFileNames[enumName] = file.name;
        }
      }
    }

    return _ReflectionCatalog(
      services: services.toList()..sort(),
      filesByName: filesByName,
      fileBytesByName: fileBytesByName,
      symbolsToFileNames: symbolsToFileNames,
    );
  }

  ServerReflectionResponse response(ServerReflectionRequest request) {
    switch (request.whichMessageRequest()) {
      case ServerReflectionRequest_MessageRequest.listServices:
        final payload = ListServiceResponse()
          ..service.addAll(
            services.map((name) => ServiceResponse()..name = name),
          );
        return _baseResponse(request)..listServicesResponse = payload;

      case ServerReflectionRequest_MessageRequest.fileContainingSymbol:
        final fileName = symbolsToFileNames[request.fileContainingSymbol];
        if (fileName == null) {
          return _errorResponse(
            request,
            code: 5,
            message: 'symbol not found: ${request.fileContainingSymbol}',
          );
        }
        return _descriptorResponse(request, fileName);

      case ServerReflectionRequest_MessageRequest.fileByFilename:
        if (!filesByName.containsKey(request.fileByFilename)) {
          return _errorResponse(
            request,
            code: 5,
            message: 'file not found: ${request.fileByFilename}',
          );
        }
        return _descriptorResponse(request, request.fileByFilename);

      case ServerReflectionRequest_MessageRequest.fileContainingExtension:
        return _errorResponse(
          request,
          code: 12,
          message: 'file_containing_extension is not implemented',
        );

      case ServerReflectionRequest_MessageRequest.allExtensionNumbersOfType:
        return _errorResponse(
          request,
          code: 12,
          message: 'all_extension_numbers_of_type is not implemented',
        );

      case ServerReflectionRequest_MessageRequest.notSet:
        return _errorResponse(
          request,
          code: 3,
          message: 'empty reflection request',
        );
    }
  }

  ServerReflectionResponse _descriptorResponse(
    ServerReflectionRequest request,
    String fileName,
  ) {
    final payload = FileDescriptorResponse()
      ..fileDescriptorProto.addAll(_descriptorClosure(fileName));
    return _baseResponse(request)..fileDescriptorResponse = payload;
  }

  List<List<int>> _descriptorClosure(String fileName) {
    final ordered = <List<int>>[];
    final visited = <String>{};

    void visit(String current) {
      if (!visited.add(current)) {
        return;
      }
      final file = filesByName[current];
      if (file == null) {
        return;
      }
      for (final dependency in file.dependency) {
        visit(dependency);
      }
      final bytes = fileBytesByName[current];
      if (bytes != null) {
        ordered.add(bytes);
      }
    }

    visit(fileName);
    return ordered;
  }

  ServerReflectionResponse _baseResponse(ServerReflectionRequest request) {
    return ServerReflectionResponse()
      ..validHost = request.host
      ..originalRequest = request;
  }

  ServerReflectionResponse _errorResponse(
    ServerReflectionRequest request, {
    required int code,
    required String message,
  }) {
    return _baseResponse(request)
      ..errorResponse = (ErrorResponse()
        ..errorCode = code
        ..errorMessage = message);
  }

  static descriptor.FileDescriptorSet _loadDescriptorSet(String protoDir) {
    final roots = _includeRoots(protoDir);
    final inputs = _protoInputs(roots);
    if (inputs.isEmpty) {
      throw StateError('no .proto files found under $protoDir');
    }

    final tempDir =
        Directory.systemTemp.createTempSync('dart-holons-reflection-');
    try {
      final outputPath = '${tempDir.path}/reflection.pb';
      final result = Process.runSync(
        'protoc',
        <String>[
          ...roots.expand((root) => <String>['-I', root]),
          '--include_imports',
          '--descriptor_set_out=$outputPath',
          ...inputs,
        ],
      );
      if (result.exitCode != 0) {
        final stderrOutput = (result.stderr ?? '').toString().trim();
        throw StateError(
          stderrOutput.isEmpty
              ? 'protoc failed while building reflection descriptors'
              : stderrOutput,
        );
      }
      return descriptor.FileDescriptorSet.fromBuffer(
        File(outputPath).readAsBytesSync(),
      );
    } finally {
      tempDir.deleteSync(recursive: true);
    }
  }

  static List<String> _includeRoots(String protoDir) {
    final roots = <String>[];

    void addRoot(String candidate) {
      final normalized = Directory(candidate).absolute.path;
      if (Directory(normalized).existsSync() && !roots.contains(normalized)) {
        roots.add(normalized);
      }
    }

    addRoot('$protoDir/api');
    addRoot('$protoDir/protos');
    addRoot(protoDir);

    var current = Directory(protoDir).absolute;
    while (true) {
      addRoot('${current.path}/_protos');
      final parent = current.parent;
      if (parent.path == current.path) {
        break;
      }
      current = parent;
    }

    return roots;
  }

  static List<String> _protoInputs(List<String> roots) {
    final inputs = <String>[];
    final seenPaths = <String>{};

    for (final root in roots) {
      final directory = Directory(root);
      if (!directory.existsSync()) {
        continue;
      }
      final files = directory
          .listSync(recursive: true)
          .whereType<File>()
          .where(
            (file) =>
                file.path.endsWith('.proto') &&
                file.uri.pathSegments.last != 'holon.proto',
          )
          .toList()
        ..sort((left, right) => left.path.compareTo(right.path));
      for (final file in files) {
        final absolutePath = file.absolute.path;
        if (seenPaths.add(absolutePath)) {
          inputs.add(
            file.path.substring(root.length + 1).replaceAll('\\', '/'),
          );
        }
      }
    }

    return inputs;
  }

  static void _indexMessage(
    descriptor.DescriptorProto message, {
    required String prefix,
    required String fileName,
    required Map<String, String> symbolsToFileNames,
  }) {
    final messageName = _qualify(prefix, message.name);
    if (messageName.isNotEmpty) {
      symbolsToFileNames[messageName] = fileName;
    }
    for (final enumType in message.enumType) {
      final enumName = _qualify(messageName, enumType.name);
      if (enumName.isNotEmpty) {
        symbolsToFileNames[enumName] = fileName;
      }
    }
    for (final nested in message.nestedType) {
      _indexMessage(
        nested,
        prefix: messageName,
        fileName: fileName,
        symbolsToFileNames: symbolsToFileNames,
      );
    }
  }

  static String _qualify(String prefix, String name) {
    if (name.isEmpty) {
      return prefix;
    }
    if (prefix.isEmpty) {
      return name;
    }
    return '$prefix.$name';
  }
}
