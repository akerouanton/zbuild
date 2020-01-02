# Generic kind parameters

Following parameters are common to many or all kinds of definition:

* [Config files - `<config_files>`](#config-files---config_files)
* [External files - `<external_files>`](#external-files---external_files)
* [Source context - `<source_context>`](#source-context---source_context)
* [System packages - `<system_packages>`](#system-packages---system_packages)
* [Healthcheck - `<healthcheck>`](#healthcheck---healthcheck)
  * [`cmd` healthcheck](#cmd-healthcheck)
  * [`fcgi` healthcheck](#fcgi-healthcheck)
  * [`http` healthcheck](#http-healthcheck)
* [Sources - `<sources>`](#sources---sources)
* [Config files - `<config_files>`](#config-files---config_files)
* [Stateful dirs - `<stateful_dirs>`](#stateful-dirs---stateful_dirs)

#### Config files - `<config_files>`

This is a map of source to destination paths of config files you want to
include in your image. Source path are relative to the build context root
directory. Destination paths could be either absolute or relative to the 
working directory used by each specialized builders. Moreover, you can use
POSIX-like parameter expansion in the destination paths. Each specialized
builder defines its own set of parameters, check their docs for more details.

Example:

```
$ tree .
.
├── docker
│   ├── nginx.conf
│   └── ...
├── docker-compose.yml
├── zbuild.lock
└── zbuild.yml
```

```yaml
# syntax=akerouanton/zbuilder:<tag>
kind: webserver

config_files:
  docker/nginx.conf: "${config_dir}/nginx.conf"
```

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

#### Source context - `<source_context>`

`source_context` takes a `context` parameter which defines the context where
`sources` are copied from. When left empty, the source context defaults to the
current [build context](https://docs.docker.com/engine/reference/commandline/build/#extended-description).

For instance, `docker build -f api.zbuild.yml .` uses the current working
directory as the build context. As such, if `source_context` is left empty in
the `api.zbuild.yml` file, the sources will be copied from the current working
directory.

Syntax:

```yaml
source_context:
  type: git
  source: <string>
  reference: <string>
```

Example:

```yaml
source_context:
  type: git
  source: git://github.com/NiR-/zbuild
  reference: v0.1
```

To ensure build consistency, when you provide a Git context, zbuild will
resolve the reference to a specific hash. As such all the builds between
two runs of `zbuild update`, will always use exactly the same commit.

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

#### Sources - `<sources>`

This is the list of source files and directories you want to copy in the image.
The paths have to be relative to the root of the source context.

```yaml
sources:
  - <string>
  - <string>
```

When merging with parent stages, all the `sources` lists are merged
together. Thus you can't remove a source dir from a parent stage (you should
reorganize your stages instead).

#### Config files - `<config_files>`

The `config_files` map takes two parameters pointing to config files in the
build context. Both paths should be relative to the context root dir (the
directory at the end of `docker build ...`).

```yaml
# This is empty by default
config_files:
  php.ini: <local_path>
  fpm.conf: <local_path>
```

Both can be set independently. When merging parent stages, each property is
overriden independently too.

#### Stateful dirs - `<stateful_dirs>`

This is the list of directories containing stateful data that should be
preserved  across container restart and deployments. zbuild marks these
directories as volumes but you stil have to configure a persistent volume when
you run the image).

Common example of such stateful dirs are: upload folders, session storage
folders, etc...

```yaml
stateful_dirs:
  - <string>
```

These paths can be either relative to the root of the project directory in the 
image (e.g. `/app`) or absolute.

When merging with parent stages, all the `stateful_dirs` are merged together.
You can't remove a stateful dir from a parent stage (you should reorganize your
stages instead).
