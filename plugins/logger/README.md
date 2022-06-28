## Sample NRI Request Logger Plugin

This plugin simply logs incoming requests and events. You can configure which
of these the plugin subscribes to. Also, if configured so this plugin can
inject an environment variable or an annotation into containers for testing
and illustrative purposes.

Note that the [differ plugin](../differ) is probably better suited for actual
debugging purposes than this simple logger.
