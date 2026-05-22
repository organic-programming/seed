import 'package:holons/holons.dart' as holons;
import 'package:holons_app/holons_app.dart';

bool _registered = false;

void ensureAppDescribeRegistered() {
  if (_registered) {
    return;
  }

  final protoDir = findAppProtoDir();
  if (protoDir == null) {
    throw StateError('failed to locate app proto directory for Describe');
  }

  final response = holons.buildDescribeResponse(protoDir: protoDir);
  ensureCoreDescribeServices(
    response,
    coaxDescription:
        "COAX interaction surface for the Gabriel Greeting app. It exposes member discovery, connection, and app-level orchestration through the same shared state the UI uses.",
  );
  holons.useStaticResponse(response);
  _registered = true;
}
