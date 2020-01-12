# zbuild

Building container images without the hassle of writing Dockerfiles.

zbuild is a high-level container image build format providing an alternative to 
Dockerfiles but still fully compatible with Docker CLI. You don't have to write
system commands to build images anymore and you can instead focus on what
matters.

[![asciicast](https://asciinema.org/a/287488.svg)](https://asciinema.org/a/287488)

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
As zbuild implements builder backends for multiple kinds of images, you have to
refer to their specific parameters.

* [php](docs/kind-php.md)
* [webserver](docs/kind-webserver.md)
* More to come soon...

Moreover, note that all zbuild files have to start with following header in order
to use zbuild to build your images:

```yaml
# syntax=akerouanton/zbuilder:test9
```

#### 2. Create or Update the lock file

zbuild uses a lock file to ensure that dependencies installed during the build
process don't change randomly from one build to another. This is in line with
the Dockerfile best-practice that consist of pinning the version of each and
every dependency installed. As such, you can update your system dependencies
like you do with most modern library/package managers: `zbuild update`.

#### 3. Build images

Finally, you can build your images using

```bash
$ docker build -f zbuild.yml -t prod .
```

## How to work on this?

#### Debug LLB DAG

```bash
$ zbuild debug-llb --target prod | buildctl debug dump-llb
```

#### Run with buildkitd

1. Start buildkit: `sudo buildkitd --debug`

2. Then run following command. Note that buildkit and docker don't share their
images, so you have to build and push using Docker before executing this command:

```bash
$ buildctl build \
    --frontend dockerfile.v0 \
    --opt target=prod \
    --opt filename=zbuild.yml \
    --local context=. \
    --local dockerfile=. \
    --output type=image,name=some-image:dev
```

#### Build and push a new version

```bash
IMAGE_TAG=v<...> make build-image push
```
