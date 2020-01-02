# NodeJS definition

* [Multi-stages and dev builds](#multi-stages-and-dev-builds)
* [Build process](#build-process)
* [Locking](#locking)
* [Assets and webserver](#assets-and-webserver)
* [Syntax](#syntax)
  * [Source context - `<source_context>`](#source-context--source-context)
  * [Derived stage - `<derived_stage>`](#derived-stage---derived_stage)
  * [Stage - `<stage>`](#stage---stage)
  * [External files - `<external_files>`](#external-files---external_files)
  * [System packages - `<system_packages>`](#system-packages---system_packages)
  * [Global NodeJS packages - `<global_packages>`](#global-packages---global_packages)
  * [Build command - `<build_command>`](#build-command---build_command)
  * [Command - `<command>`](#command---command)
  * [Config files - `<config_files>`](#config-files---config_files)
  * [Sources - `<sources>`](#sources---sources)
  * [Stateful dirs - `<stateful_dirs>`](#stateful-dirs---stateful_dirs)
  * [Healthcheck - `<healthcheck>`](#healthcheck---healthcheck)
* [Full example](#full-example)

A [full example](#full-example) is available at the end of this page, but you
can also take a look at following examples:

*TODO*

## Multi-stages and dev builds

Using Docker for dev and prod purpose is not exactly the same thing. In the
first case, you generally want a minimal image to accelerate the build process
by installing just what's needed to run your project (e.g. nodejs interpreter).
With such images, bind-mounts are generally used to not rebuild the image after
each and every code change. As such, zbuild doesn't install your dependencies
in dev builds.

Moreover, if you're used to use docker and have already written Dockerfiles to
build frontend projects, you probably have used the multi-stage feature to
build your assets in a first stage and then copy them into a second stage based
on a webserver.

zbuild helps you there:

1. it supports multi-stage workflows, so you can easily create a base config
and derive it to make specialized stages. When the build process kicks off, the
final build config is resolved by merging each stage with its parent, until the
`base` stage is found ;

2. you can also mark stages as dev. This will build lighter images that include
only what's needed to start a container with bind-mounts (no npm/yarn install,
no autoload dump, no post install steps, etc...) ;

As an example here're two visual representations of the build steps executed
by zbuild. The first example has been generated from `dev` stage:

![](diagrams/nodejs-stage-dev.png)

And this one has been generated from a non-dev stage (these come from zbuild
test suite):

![](diagrams/nodejs-stage-prod.png)

## Build process

The image build process for nodejs definitions have following steps:

* Detect whether npm or yarn should be used by checking if a
`package-lock.json` or a `yarn.lock` exists in the build context ;
* Install system packages ;
* Copy external files ;
* Create /app directory ;
* Declare uid 1000 as the default user ;
* Install global packages ;

Moreover, if the stage is non-dev (see below), following steps are also applied:

* Install project packages with yarn or npm ;
* Copy sources ;
* Copy config files ;
* Run the build command ;

## Locking

When using `zbuild update` to create or update your lockfile, the base image
digest is resolved and for each stage, system packages are pinned to a specific
version.

For the base image, zbuild uses either `version` parameter to determine what's
the base image or it directly relies on `base` parameter (see below).
In both cases, the digest of the image is resolved. As such, even if the image
tag used changes, zbuild continue to use the same locked verison, until the
next run of `zbuild update`.

## Assets and webserver

When you build frontends and want to serve it through a webserver of your own,
you generally want to build your SPA and then embed the build result (a.k.a the
build artifact) into a webserver like nginx. To help you there, zbuild lets you
embed a webserver definition in a nodejs zbuildfile:

```yaml
kind: nodejs
version: 12
frontend: true

sources:
  - src/

stages:
  dev:
    config_files:
      .env.dev: .env
  prod:
    config_files:
      .env.prod: .env

webserver:
  type: nginx
  config_files:
    docker/nginx/nginx.conf: "${config_dir}/nginx.conf"
  assets:
    - from: build/
      to: /app/build/
```

For more details about webserver definition, see [here](kind-webserver.md).

## Syntax

zbuildfiles with nodejs kind have following structure:

```yaml
# syntax=akerouanton/zbuilder:<tag>
kind: nodejs

base: <string> # (required if version is empty)
version: <string> # (required if base is empty)
alpine: <bool> # (default: true)
frontend: <bool> # (default: false)

source_context: <context>

<stage>

stages:
    <stage_name>: <derived_stage>
```

When the `version` parameter is provided, the base image is defined by this
template: `docker.io/library/node:<version>-buster`.

You can also provide your own `base` image. In that case, you don't need to
define `version` and `alpine` parameters. However, note that zbuild expects the
base image to contain at least the NodeJS package manager of your choice.

You can define the `base` stage at the root of the destination. Subsequent
stages defined `stages` will then inherit parameters from the `base` stage.

See a [full example below](#full-example).

#### Source context - `<source_context>`

See [here](generic-parameters.md#source-context--source-context).

#### Derived stage - `<derived_stage>`

Derived stages have exactly the same properties as [Stage](#stage-stage), but
they can take two additional parameters:

```yaml
from: <stage_name> # Defaults to "base"
dev: <bool> # Whether this is a dev stage.
<stage>
```

#### Stage - `<stage>`

The stage parameters are the core of the NodeJS definition format. It groups
all the parameters that can be both in the base definition (the `base` stage)
and in subsequent stage definitions.

```yaml
external_files: <external_files>
system_packages: <system_packages>
global_packages: <global_packages>
build_command: <string>
command: <command>
config_files: <config_files>
sources: <sources>
stateful_dirs: <stateful_dirs>
healthcheck: <healthcheck>
```

#### External files - `<external_files>`

See [here](generic-parameters.md#external-files---external_files).

#### System packages - `<system_packages>`

See [here](generic-parameters.md#system-packages---system_packages).

#### Global NodeJS packages - `<global_packages>`

The `global_packages` parameter takes a map of javascript packages and version
constraints. These dependencies are installed using `yarn global add` or
`npm install -g` (see [Build process](#build-process)).

Example:

```yaml
global_packages:
  puppeteer: "1.10.0"
```

#### Build command - `<build_command>`

The `build_command` is used when building non-dev stages to run some
transpilers and/or bundlers (eg. Webpack, Rollup, etc...). See
[Build process](#build-process).

#### Command - `<command>`

The `command` parameter defines which command should be run when starting a
container from the image you're building. This is valid for both dev and
non-dev stages, but is mostly useful in the latter case.

#### Config files - `<config_files>`

See [here](generic-parameters.md#config-files---config_files).

#### Sources - `<sources>`

See [here](generic-parameters.md#sources---sources).

#### Stateful dirs - `<stateful_dirs>`

See [here](generic-parameters.md#stateful-dirs---stateful_dirs).

#### Healthcheck - `<healthcheck>`

The `healthcheck` parameter can be used to preconfigure container healthcheck
for this image. For nodes kind, it's either of `http` or `cmd` type. See
[here](generic-parameters.md#healthcheck) for more details about healthcheck
parameter.

The default healthcheck for nodejs kind is:

```yaml
healthcheck:
  type: http
  interval: 10s
  timeout: 1s
  retries: 3
  fcgi:
    path: /ping
    expected: pong
```

## Full example

```yml
# syntax=akerouanton/zbuilder:v0.1
kind: nodejs
version: lts
alpine: true
frontend: true

build_command: npm run build

sources:
  - public/
  - src/

stages:
  dev:
    system_packages:
      chromium: "*"
    global_packages:
      puppeteer: "1.10.0"
  preprod:
    config_files:
      .env.preprod.dist: .env
  prod:
    config_files:
      .env.prod.dist: .env
```
