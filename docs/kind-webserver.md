## Webserver definition

The webserver builder can be used to create a container image of a webserver.
It supports two different ways to invoke it:

1. alone, with its own zbuildfile (e.g. if you want to only serve an API with
no assets) ;
2. integrated within another builder (e.g. to add assets built by another
zbuildfile) ;

* [Syntax](#syntax)
  * [Webserver type - `<webserver_type>`](#webserver-type---webserver_type)
  * [System packages - `<system_packages>`](#system-packages---system_packages)
  * [Config files - `<config_files>`](#config-files---config_files)
  * [Healthcheck - `<healthcheck>`](#healthcheck---healthcheck)
  * [Assets - `<assets>`](#assets---assets)

## Syntax

Webserver definitions have following structure:

```yaml
kind: webserver

type: <webserver_type> # (default: nginx)
system_packages: <system_packages>
config_files: <config_files>
healthcheck: <bool>
assets: <assets>
```

##### Webserver type - `<webserver_type>` (default: `nginx`)

This parameter defines which webserver you want to use for this image. Only
`nginx` is supported for now.

##### System packages - `<system_packages>`

See [here](generic-parameters.md#system-packages---system_packages).

##### Config files - `<config_files>`

See [here](generic-parameters.md#config-files)

This builder defines no working directory but instead reuses the working
directory of the base image.

Following parameters are available for expansion:

* `${config_dir}`: points to `/etc/nginx` ;

#### Healthcheck - `<healthcheck>`

The `healthcheck` parameter can be used to preconfigure Docker healthcheck for
this image. For webserver definitions, the healthcheck can either be of type
`http` or `cmd`. See [here](generic-parameters.md#healthcheck) for more details
about healthcheck parameter.

`http` healthchecks are using `curl` and corresponding package is automatically
added to your `system_packages`.

The default healthcheck for nginx is:

```yaml
healthcheck:
  type: http
  interval: 10s
  timeout: 1s
  retries: 3
  fcgi:
    path: /_ping
    expected: pong
```

You still have to properly configure your webserver to expose a ping/pong
healthcheck on `/_ping`.

Example `nginx.conf`:

```
http {
    server {
        # ...

        location = /_ping {
            access_log off;
            allow 127.0.0.1;
            deny all;
            return 200 "pong";
        }
    }
}
```

##### Assets - `<assets>`

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
