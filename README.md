# Netwrangler

Netwrangler is a one-shot network interface configuration utility that
is mostly compatible with https://netplan.io configuration files.  Key
differences are:

* It only supports systemd-networkd and old-style Redhat network
  configurations as output formats.  Debian style is a planned on, and
  NetworkManager style is a lower priority.
* No support for configuring wireless interfaces.  This tool is mainly
  intended for servers and other devices that do not have wireless
  interfaces.
* No daemons, dynamic configuration, or other long-lived operations.
  This tool is intended to be run as part of device provisioning,
  where we expect to set the desired network interface config once and
  then forget about it until it is time to reprovision the box.
* No support for hierarchical config files.  We use the netplan.io
  YAML for its schema, not to allow additional layered customization.
* No support for NIC renaming or MAC address reassignment.  Support
  may be added at a later date.
* No support for per-interface backend renderers.  This just doesn't
  seem like a good idea if you don;t care about dynamic interface
  reconfiguration.
