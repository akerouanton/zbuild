# PHP definitions

* [Multi-stages and dev builds](#multi-stages-and-dev-builds)
* [Config Inference](#config-inference)
* [Syntax](#syntax)
* [Integrations](#integrations)
* [Example](#example)

## Multi-stages and dev builds

Using Docker for dev and prod purpose is not exactly the same thing. In the
first case, you probably want some development tools, an unoptimized PHP
autoloader, bind-mounts between your container and local sources etc... Whereas
for your prod environment, you probably want an optimized autoloader, no
bind-mounts, etc...

Moreover, if you're used to use docker and write Dockerfiles to build complex
PHP project, you probably have used the multi-stage feature to build the FPM
server along with some workers, all from a single `Dockerfile`, using a common
stage.

zbuild helps you there:

1. it supports multi-stage workflows, so you can easily create a base config
and derive it to make specialized stages ;

2. you can also mark stages as dev. This will build lighter images that include
only what's needed to start a container with bind-mounts (no composer install,
no autoload dump, no post install steps, etc...) ;

## Config Inference

## Syntax

zbuild files with php types have following structure:

```yaml
# syntax=akerouanton/zbuilder:<tag>
kind: php

base: <string>
version: <string>
infer: <bool>

<stage>

stages:
    <stage_name>: <derived_stage>
```

#### Derived stage - `<derived_stage>`

Derived stages have exactly the same properties as [Stage](#stage-stage), but they
can take two additional parameters:

```yaml
derived_from: <stage_name> # Defaults to "base"
dev: <bool> # Whether this is a dev stage.
<stage>
```

#### Stage - `<stage>`

The stage parameters are the core of the PHP definition format. It groups all the
parameters that can be both in the base definition (the `base` stage) and in subsequent stage definitions.

```yaml
fpm: <bool>
command: <string>
healthcheck: <bool> # False by default
external_files: <external_files>
system_packages: <system_packages>
extensions: <extensions>
config_files: <config_files>
composer_dump: <composer_dump>
source_dirs: <source_dirs>
extra_scripts: <extra_scripts>
integrations: <integrations>
stateful_dirs: <stateful_dirs>
post_install: <post_install>
```

#### Healthcheck - `<healthcheck>`

The `healthcheck` parameter is a bool that can be used to preconfigure Docker healthcheck for
this image (interval: 10s, timeout: 1s, retries: 3). This healthcheck sends a FCGI request to the FPM backend with the route `/_ping.php`. You have to set following parameters in your `fpm.conf` file:

```
[www]
ping.path = /_ping.php
```

#### External files - `<external_files>`

This is for advanced usage.

## System packages - `<system_packages>`



#### Extensions - `<extensions>`

The `extensions` parameter takes a map of extension names and versions:

```yaml
# This is empty by default
extensions:
  intl: "*"
  pdo_mysql: "*"
```

When merging extensions from parent stages, all the maps of extensions are merged together. You cannot remove/disable an extension from a parent stage.

#### Config files - `<config_files>`

The `config_files` map takes two parameters pointing to config files on the host. Both paths should be relative to the context root dir (the directory at the end of `docker build ...`).

```yaml
# This is empty by default
config_files:
  php.ini: <local_path>
  fpm.conf: <local_path>
```

Both can be set independently. When merging parent stages, each property is overriden independently too.

#### `composer dump` flags - `<composer_dump>`

This map takes two parameters matching the optimization flags you can pass to `composer dump. See ["Composer - Autoloader Optimization"](https://getcomposer.org/doc/articles/autoloader-optimization.md#autoloader-optimization) if you're not sure what these parameters mean.

```yaml
composer_dump:
  apcu: <bool> # Default value for base stage: false
  classmap_authoritative: <bool> # Default value for base stage: true
```

When merging parent stages, the whole map (if declared) erase parent values.

#### source_dirs - `<source_dirs>`

This is the list of source directories you want to copy in the image.

```yaml
source_dirs:
  - <local_path>
  - <local_path>
```

When merging with parent stages, all the `source_dirs` lists are merged together. Thus you can't remove a source dir from a parent stage (you should reorganize your stages instead).

#### extra_scripts - `<extra_scripts>`

This is the list of scripts outside of the `source_dirs` that should be
included in the image. All the script paths should be relative

#### integrations - `<integrations>`

This is a list of integrations that should be enabled. See [Integrations](#integrations) below for more details about what integrations are available.

```yaml
integrations:
  - <string>
```

When merging with parent stages, all the `interactions` lists are merged together. You can't remove an integration from a parent stage (you should reorganize your stages instead).

#### stateful_dirs - `<stateful_dirs>`

This is the list of directories containing stateful data when you run the image.
These "stateful data" should be preserved restart and deployment. As such,
zbuild marks these directories as volumes (you stil have to configure a
persistent volume when you run the image).

Common example of such stateful dirs are: upload folders, PHP session storage
folders, etc...

```yaml
stateful_dis:
  - <container_path>
```

When merging with parent stages, all the `stateful_dirs` are merged together.
You can't remove a stateful dir from a parent stage (you should reorganize your
stages instead).

#### post_install - `<post_install>`


#### Local paths - `<local_paths>`

Some of [`<stage>` properties][#stage-properties] are local paths. All these paths should be relative to the root dir of the build context (= relative to the directory at the end of `docker build ...`).

## Example

```yml
# syntax=akerouanton/zbuilder:test3
kind: php
fpm: true
version: 7.4.0

extensions:
  intl: "*"
  pdo_mysql: "*"
  soap: "*"

# These parameters are automatically set by zbuild using the "symfony" integration, so there're not needed in this case.
# source_dirs:
#   - './app'
#   - './src'
# extra_scripts:
#   - './bin/console'
#   - './web/app.php'

stateful_dirs:
  - './web/uploads'

config_files:
  fpm.conf: 'docker/app/fpm.conf'

integrations:
  - symfony

stages:
  dev:
    config_files:
      php.ini: 'docker/app/php.dev.ini'

  prod:
    config_files:
      php.ini: 'docker/app/php.prod.ini'
    extensions:
      apcu: "*"
      opcache: "*"
    healthcheck: true
    integrations:
      - blackfire

  worker:
    derive_from: prod
    healthcheck: false
```
