# Generic kind parameters

Following parameters are common to many or all kinds of definition:

* [External files - `<external_files`](#external-files---external_files)
* [System packages - `<system_packages>`](#system-packages---system_packages)
* [Healthcheck - `<healthcheck>`](#healthcheck---healthcheck)
  * [`cmd` healthcheck](#cmd-healthcheck)
  * [`fcgi` healthcheck](#fcgi-healthcheck)
  * [`http` healthcheck](#http-healthcheck)

#### External files - `<external_files>`

This parameter allows you to download files through HTTP or HTTPS to put them
at a specific path in your image. These files might be compressed or
uncompressed, and you might either decompress a full archive or a single path
from that archive. It supports tar archives, either uncompressed or compressed
with gzip, bzip2 or xz.

Moreover, it supports checksum verification.

When merging this parameter with a parent stage, the list of external files 
of the parent is appended to the child's list. As such, you can't remove a
file from a parent declaration.

```yaml
external_files:
  - url: <string>
    compressed: <bool>
    pattern: <string>
    destination: <string>
    checksum: <string>
    mode: <octal_number>
    owner: <string>
```

Example for an uncompressed file:

```yaml
external_files:
  - url: https://github.com/NiR-/fcgi-client/releases/download/v0.2.0/fcgi-client.phar
    destination: /usr/local/bin/fcgi-client
    mode: 0750
    owner: 1000:1000
```

Example for extracting a single file from an archive:

```yaml
external_files:
  - url: https://blackfire.io/api/v1/releases/probe/php/linux/amd64/72
    compressed: true
    pattern: blackfire-*.so
    destination: /usr/local/lib/php/extensions/no-debug-non-zts-20190902/blackfire.so
    mode: 0644
```

#### System Packages - `<system_packages>`

This parameter is a map of system packages requirements. System packages are
packages that can be installed using the package manager available in the base
image (only apt is supported as of now since Alpine is using rolling updates).

Package requirements can be either a `*` or a specific version. When a wildcard
is used, the latest version available is locked. For more details about version
locking, see the doc pages for each kind.

```yaml
system_packages:
  <string>: <string>
```

Example:

```yaml
system_packages:
  curl: "*"
  chromium: "78.0.3904.108-1~deb10u1"
```

#### Healthcheck - `<healthcheck>`

All kinds have a `healthcheck` parameter with their own default value and their
own set of allowed healthcheck types. However all these `healthcheck`
parameters have the same shape:

```yaml
healthcheck:
  type: <string>
  interval: <duration>
  timeout: <duration>
  retries: <integer>
  <healthcheck_type>: <specialized_parameters>
```

`<duration>` types are integers followed by duration suffix `ms` or `s`.

Each type of healthcheck have its own set of specialized parameters:

* [`cmd` healthcheck type](#cmd-healthcheck)
* [`fcgi` healthcheck type](#fcgi-healthcheck)
* [`http` healthcheck type](#http-healthcheck)

##### `cmd` healthcheck

```yaml
healthcheck:
  type: cmd
  cmd:
    shell: <bool>
    command: <list_of_string>
```

The `shell` parameter indicates whether the given command should run inside a
shell session. The `command` parameter is the command to run as a healthcheck
prober.

##### `fcgi` healthcheck

```yaml
healthcheck:
  type: fcgi
  fcgi:
    path: /ping
    expected: pong
```

The `fcgi` healthcheck type uses [fcgi-client](https://github.com/NiR-/fcgi-client)
to send FCGI requests to a php-fpm server listening on `127.0.0.1:9000` with
the given `path` parameter. The healthcheck prober expects the FCGI request
to return the given `expected` parameter as response body.

##### `http` healthcheck

```yaml
healthcheck:
  type: http
  http:
    path: /ping
    expected: pong
```

The `http` healthcheck type uses `curl` to send a request to the given `path` to
`127.0.0.1`. The healthcheck prober expects the HTTP request to return the
given `expected` parameter.
