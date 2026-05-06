% containers-certs.d 5 Directory for storing custom container-registry TLS configurations

# NAME
containers-certs.d - Directory for storing custom container-registry TLS configurations

# DESCRIPTION
A custom TLS configuration for a container registry can be configured by creating a directory named after the registry `host`[`:port`] (for example, `my-registry.com:5000`) in one of the following locations.
Directories are consulted in this order (highest priority first):

- For both rootful and rootless:
  - `$XDG_CONFIG_HOME/containers/certs.d/` (or `$HOME/.config/containers/certs.d/` if `XDG_CONFIG_HOME` is unset)
- For rootful (UID == 0):
  - `/etc/containers/certs.rootful.d/`
- For rootless (UID > 0):
  - `/etc/containers/certs.rootless.d/<UID>/`
  - `/etc/containers/certs.rootless.d/`
- For both rootful and rootless:
  - `/etc/containers/certs.d/`
- For rootful (UID == 0):
  - `/usr/share/containers/certs.rootful.d/`
- For rootless (UID > 0):
  - `/usr/share/containers/certs.rootless.d/<UID>/`
  - `/usr/share/containers/certs.rootless.d/`
- For both rootful and rootless:
  - `/usr/share/containers/certs.d/`
- Compatibility fallback:
  - `/etc/docker/certs.d/`

The port part presence / absence must precisely match the port usage in image references,
e.g. to affect `podman pull registry.example/foo`,
use a directory named `registry.example`, not `registry.example:443`.
`registry.example:443` would affect `podman pull registry.example:443/foo`.

## Directory Structure
A certs directory can contain one or more files with the following extensions:

* `*.crt`  files with this extensions will be interpreted as CA certificates
* `*.cert` files with this extensions will be interpreted as client certificates
* `*.key`  files with this extensions will be interpreted as client keys

Note that the client certificate-key pair will be selected by the file name (e.g., `client.{cert,key}`).
An exemplary setup for a registry running at `my-registry.com:5000` may look as follows:
```
/etc/containers/certs.d/    <- Certificate directory
└── my-registry.com:5000    <- Hostname[:port]
   ├── client.cert          <- Client certificate
   ├── client.key           <- Client key
   └── ca.crt               <- Certificate authority that signed the registry certificate
```

# HISTORY
Feb 2019, Originally compiled by Valentin Rothberg <rothberg@redhat.com>
