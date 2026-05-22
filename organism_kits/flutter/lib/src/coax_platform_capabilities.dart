import 'package:holons/holons.dart' show AppPlatformCapabilities;

import 'coax_configuration.dart';

extension CoaxPlatformCapabilities on AppPlatformCapabilities {
  List<CoaxServerTransport> get coaxServerTransports => supportsUnixSockets
      ? CoaxServerTransport.values
      : const [CoaxServerTransport.tcp];
}
