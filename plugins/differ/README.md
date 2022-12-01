## NRI Differ Plugin

The differ plugin can be injected before, after or between other NRI plugins
to track and show what changes these other plugins request to containers.
The plugin can register itself multiple times at multiple indices, so a single
differ instance can be used to track and show step-by-step all the changes
made to a container.

## Testing

You can test this plugin by registering it to the desired indices (for
instance `nri-differ --indices 00,20,99 --yaml`) in addition to your other
plugins that make changes to containers, then starting some containers
and examining the results. You should see container modifications printed
as yaml-diffs. Make sure you properly inject/register `differ` both at the
front of the plugin chain and after any other plugin.
