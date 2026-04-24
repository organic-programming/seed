# MetricsView

`MetricsView(controller:)` renders counters, gauges, and histograms from
`MetricsController`. Gauge rows include a `SparklineView` over the controller's
last 30 snapshots.

The view uses stock SwiftUI text, stacks, and the current accent color. Embed it
inside a scrollable or full-height detail area; the view already provides its
own scrolling.

