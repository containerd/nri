## NRI v0.1.0 Compatibility Plugin

This plugin aims to emulate the original NRI API and functionality. It uses
the v0.1.0 NRI Client package to load any v0.1.0 plugins and hook them into
the start and stop lifecycle events of pods and containers. The data passed
to the v0.1.0 plugins is reconstructed from the NRI event data this plugin
receives. It is not guaranteed to be 100% identical to the data provided by
the original interface but believed to be close enough to allow old plugins
to function.

## Testing

You can enable backward compatibility by compiling this plugin, installing,
then linking it among the automatically launched plugins.

```
# Compile v0.1.0 adapter plugin and install it.
make $(pwd)/build/bin/v010-adapter
sudo cp build/bin/v010-adapter /usr/local/bin
# Link it among the automatically launched plugins.
sudo mkdir -p /opt/nri/plugins
sudo ln -s /usr/local/bin/v010-adapter /opt/nri/plugins/00-v010-adapter
# Make sure NRI is enabled.
sudo mkdir -p /etc/nri
sudo touch /etc/nri/nri.conf
```

Once this is done, any NRI v0.1.0 plugin binaries found in `/opt/nri/bin`
should be executed for each RunPodSandbox, StopPodSandbox, StartContainer
and StopContainer events.
