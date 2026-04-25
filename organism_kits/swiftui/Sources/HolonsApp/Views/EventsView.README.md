# EventsView

`EventsView(controller:)` renders the event ring with event-type badges, origin
slug, timestamp, optional chain text, and payload fields.

The badge palette follows the host app's accent and system semantic colors. Use
the controller's gate to filter events by family state before they reach the
view.

