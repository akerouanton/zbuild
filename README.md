# zbuild: Building Docker images without the hassle of writing system commands

zbuild is a high-level Buildkit syntax provider. That means, it provides
a syntax for defining and building container images with higher-level concepts,
such that you don't have to write system commands anymore and you can instead
focus on what matters.

## How to use?

#### 0. Enable Docker Experimental features

As of now (2019-11-14), Buildkit is integrated into Docker but as an
experimental feature. Thus, you need to enable experimental features to use
Buildkit and this syntax provider.

If you are using Docker v18.06 or later, BuildKit mode can be enabled by
setting export DOCKER_BUILDKIT=1 on the client side. Docker v18.06 also
requires the daemon to be running in experimental mode.

#### 1. Write zbuild files

Instead of writing Dockerfiles, you have to write zbuild files in YAML format.
As zbuild implements builder backends for multiple types of images, you have to
refer to their specific parameters.

* [php](docs/php-parameters.md)
* More to come soon...

Moreover, whatever backend you want to use, you have to include the same header
as following example. This is used by Buildkit to know which syntax provider to use.

```yaml
# syntax=akerouanton/webdf-builder:test9
type: php
fpm: true
version: 7.0.29

extensions:
  intl: "*"
  pdo_mysql: "*"
  soap: "*"

stateful_dirs:
  - './web/uploads'

config_files:
  fpm.conf: 'docker/app/fpm.conf'

integrations:
  - symfony

stages:
  dev:
    dev: true
    config_files:
      php.ini: 'docker/app/php.dev.ini'

  prod:
    healthcheck: true
    config_files:
      php.ini: 'docker/app/php.prod.ini'
    extensions:
      apcu: "*"
      opcache: "*"
    integrations:
      - blackfire

  worker:
    derive_from: prod
    healthcheck: false
```

#### 2. Create or Update the lock file

zbuild uses a lock file to ensure that dependencies installed during the build
process don't change randomly from one build to another. This is in line with
the Dockerfile best-practice that consist of pinning the version of each and
every dependency installed. As such, you can update your system dependencies
like you do with most modern package managers: `zbuild update`.

#### 3. Build images

Finally, you can build your images using

```bash
$ docker build -f webdf.yml -t prod .
```

## How to work on this?

#### Debug LLB DAG

```bash
$ webdf debug-llb --target prod | buildctl debug dump-llb
```

#### Run with buildkitd

1. Start buildkit: `sudo buildkitd --debug`

2. Then run following command. Note that buildkit and docker don't share their
images, so you have to build and push using Docker before executing this command:

```bash
$ buildctl build \                         
    --frontend gateway.v0 \
    --opt source=akerouanton/webdf-builder:test9 \
    --opt target=prod \
    --local context=. \
    --output type=image,name=some-image:dev
```

#### Build and push a new version

```bash
IMAGE_TAG=v<...> make build-image push
```
