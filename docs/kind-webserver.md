## Webserver definition

The webserver builder can be used to create a container image of a webserver.
It supports two different ways to invoke it:

1. alone, with its own zbuildfile (e.g. if you want to only serve an API with
no assets) ;
2. integrated within another builder (e.g. to add assets built by another
zbuildfile) ;

## Syntax

zbuildfiles with webserver types have following structure:

```yaml
kind: webserver

type: <string> # (default: nginx)
system_packages: <map[string]string>
config_file: <string> # (required)
healthcheck: <bool> # (default: true)
assets: <assets>
```

##### `type` - default: `nginx`

This parameter defines which webserver you want to use for this image. Only
`nginx` is supported for now.

##### `system_packages` - not required

This parameter can be used to install custom system packages in the image. It's
a map of package names as keys and version constraints as values.

Example:

```yaml
# syntax=akerouanton/zbuilder:<tag>
kind: webserver

system_packages:
  curl: *
```

System packages are pinned to a specific version in the lockfile with the help
of `zbuild update`. See [here](/README.md#2-create-or-update-the-lock-file) for more details.

##### `config_file` - **required**

This is the path to your local `nginx.conf` config file.

Example:

```
$ tree .
.
├── docker
│   └── nginx.conf
├── docker-compose.yml
├── zbuild.lock
└── zbuild.yml

$ cat zbuild.yml
# syntax=akerouanton/zbuilder:<tag>
kind: webserver

config_file: nginx.conf
```

##### `healthcheck` - default: `true`

Whether the image should have a preconfigured healthcheck. It automatically
adds curl to the list of system packages to install but you still have to
properly configure your webserver to expose a ping/pong healthcheck on
`/_status` (the endpoint used is not configurable).

Example `nginx.conf`:

```
server {
    # ...

    location = /_status {
        stub_status;
        access_log off;

        allow 127.0.0.1;
        deny all;
    }
}
```

##### `assets`

This parameter can only be used when the webserver builder is called by another
builder (e.g. within a zbuildfile with php or nodejs kind). It's a list of 
`from`/`to` tuples. The `from` parameter tells where the assets to copy are in
the base image. The `to` parameters tells where the assets should be copied
into the final image (the one with the webserver).

Example:

```yaml
# syntax=akerouanton/zbuilder:<tag>
kind: php

webserver:
  assets:
    - from: /app/public
      to: /var/www/html
```

If you build this zbuildfile by targeting `webserver-prod`, the assets in
`/app/public` from the final php image will be copied to `/var/www/html`
in the webserver image.
