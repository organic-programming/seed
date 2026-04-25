# LogConsoleView

`LogConsoleView(controller:)` renders live `ConsoleController` entries with a
level picker, text filter, level badge colors, origin slug, chain text, message,
and fields.

It inherits app typography and accent color. Override badge color behaviour by
wrapping or replacing the view and feeding the same controller. The console is
intended for flexible full-width containers.

