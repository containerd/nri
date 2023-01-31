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
# Make sure NRI is enabled in containerd:
systemctl stop containerd
cp /etc/containerd/config.toml /etc/containerd/config.toml.orig
containerd config dump > /etc/containerd/config.toml
$EDITOR /etc/containerd/config.toml
#  Change `disable = true` to `disable = false` in the
#      `[plugins."io.containerd.nri.v1.nri"]` section.
systemctl start containerd
# Verify that NRI is enabled.
[ -e /var/run/nri.sock ] && echo "NRI is enabled" || echo "NRI is disabled"
# Link the adapter plugin among the automatically launched plugins.
sudo mkdir -p /opt/nri/plugins
sudo ln -s /usr/local/bin/v010-adapter /opt/nri/plugins/00-v010-adapter
```

Once this is done, any NRI v0.1.0 plugin binaries found in `/opt/nri/bin`
which are also enabled in /etc/nri/conf.json should be executed for each
RunPodSandbox, StopPodSandbox, StartContainer and StopContainer events.
